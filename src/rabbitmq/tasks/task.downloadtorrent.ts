import { Channel, ChannelWrapper } from 'amqp-connection-manager';
import { ConsumeMessage } from 'amqplib';
import { ITorrent, TorrentPath, IVideo, QueueName } from '../../@types';
import {
  IConvertVideoMessageContent,
  IMoveFilesMessageContent,
  ITorrentDownloadStatusMessageContent,
} from '../../@types/message';
import logger from '../../config/logger';
import client from '../../config/webtorrent';
import { allowedExt, convertableExt, getFileOutputPath, getMessageContent } from '../../utils/misc';
import { updateTorrentInfo } from '../../utils/query';

export const downloadTorrent =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  // eslint-disable-next-line sonarjs/cognitive-complexity
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    try {
      const addedTorrent: ITorrent = getMessageContent<ITorrent>(message);
      logger.info({ message: 'Received new torrent to download..', addedTorrent });

      client.add(addedTorrent.magnet, { path: TorrentPath.TMP }, async torrent => {
        torrent.on('error', () => {
          logger.error(`Torrent error ack msg: ' ${addedTorrent} `);
          channel.ack(message);
        });
        const videofiles = torrent.files
          .map(file => {
            const ext = file.name.split('.').pop() || '';
            const isVideoFile = allowedExt.has(ext);
            const isConvertable = convertableExt.has(ext);
            if (!isVideoFile) {
              logger.info(`found file that's not video file ${file.name}`);
              // file.deselect();
            }
            return {
              name: file.name,
              path: file.path,
              size: file.length,
              ext,
              isConvertable,
              status: 'downloading',
            } as IVideo;
          })
          .filter(file => allowedExt.has(file.ext));

        if (videofiles.length === 0) {
          torrent.destroy({ destroyStore: true });
          logger.info({ message: 'no video files found, deleting torrent', addedTorrent });
          // eslint-disable-next-line no-underscore-dangle
          updateTorrentInfo(addedTorrent._id, { status: 'error', isMedia: false });
          channel.ack(message);
        } else {
          const { name, infoHash, length: size } = torrent;
          const isMultiVideos = videofiles.length > 1;
          const SavedTorrent = await updateTorrentInfo(addedTorrent._id, {
            name,
            infoHash,
            size,
            isMultiVideos,
            files: videofiles,
            status: 'downloading',
          });
          if (!SavedTorrent) {
            logger.error(`torrent not saved ${addedTorrent}`);
            channel.ack(message);
            return;
          }

          //* publish a message to track this torrent's download status and save td db
          publisherChannel.sendToQueue(QueueName.TRACK_TORRENT, {
            torrentID: SavedTorrent._id,
            torrentInfoHash: SavedTorrent.infoHash,
          } as ITorrentDownloadStatusMessageContent);
          torrent.on('done', async () => {
            await updateTorrentInfo(SavedTorrent._id, { status: 'done' });
            const convertableVideoFiles = SavedTorrent.files.filter(file => file.isConvertable);
            const nonConvertableVideoFiles = SavedTorrent.files.filter(file => !file.isConvertable);
            if (convertableVideoFiles.length > 0) {
              //* sending all convertable files to convert-video queue
              convertableVideoFiles.map(file =>
                publisherChannel.sendToQueue(QueueName.CONVERT_VIDEO, {
                  torrentID: SavedTorrent._id,
                  ...file,
                } as IConvertVideoMessageContent)
              );
            }
            //* sending all nonconvertable files to file-move queue to move them to download folder
            await Promise.all(
              nonConvertableVideoFiles.map(file =>
                publisherChannel.sendToQueue(QueueName.FILE_MOVE, {
                  src: file.path,
                  dest: getFileOutputPath(file.name, TorrentPath.DOWNLOAD),
                  torrentID: SavedTorrent._id,
                  fileSlug: file.slug,
                } as IMoveFilesMessageContent)
              )
            );
            channel.ack(message);
            logger.info(`${SavedTorrent.name} torrent downloaded now deleting torrent`);
            //! this might leave usless files in tmp folder
            torrent.destroy();
          });
        }
      });
    } catch (error) {
      logger.error(error);
      channel.ack(message);
    }
  };
