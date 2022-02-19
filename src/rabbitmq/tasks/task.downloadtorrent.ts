/* eslint-disable unicorn/consistent-destructuring */
import { Channel, ChannelWrapper } from 'amqp-connection-manager';
import { ConsumeMessage } from 'amqplib';
import { Torrent } from 'webtorrent';
import { ITorrent, TorrentPath, IVideo, QueueName } from '../../@types';
import { IConvertVideoMessageContent, IMoveFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import client from '../../config/webtorrent';
import { allowedExt, convertableExt, getFileOutputPath, getMessageContent } from '../../utils/misc';
import { getTorrentBySlug, updateTorrentFileConvertable, updateTorrentInfo } from '../../utils/query';
import { isFileConvertable } from '../../utils/videoConverter';

const handleCompletedTorrent = async (
  torrentInClient: Torrent,
  downloadedTorrent: ITorrent,
  publisherChannel: ChannelWrapper,
  consumerChannel: Channel,
  message: ConsumeMessage
) => {
  await updateTorrentInfo(downloadedTorrent._id, { status: 'done' });

  await Promise.allSettled(
    downloadedTorrent.files.map(file => {
      return isFileConvertable(file).then(isConvertable => {
        updateTorrentFileConvertable(downloadedTorrent._id, file.slug, isConvertable);
      });
    })
  ).catch(error => {
    logger.error(error);
  });

  const UpdatedTorrent = await getTorrentBySlug(downloadedTorrent.slug);
  if (!UpdatedTorrent) {
    logger.error(`torrent not found after updating convertable data ${downloadedTorrent.name}`);
    consumerChannel.ack(message);
    return;
  }

  const convertableVideoFiles = UpdatedTorrent.files.filter(file => file.isConvertable);
  const nonConvertableVideoFiles = UpdatedTorrent.files.filter(file => !file.isConvertable);

  if (convertableVideoFiles.length > 0) {
    //* sending all convertable files to convert-video queue
    convertableVideoFiles.map(file =>
      publisherChannel.sendToQueue(
        QueueName.CONVERT_VIDEO,
        {
          torrentID: downloadedTorrent._id,
          ...file,
        } as IConvertVideoMessageContent,
        { persistent: true }
      )
    );
  }
  if (nonConvertableVideoFiles.length > 0) {
    //* sending all nonconvertable files to file-move queue to move them to download folder
    await Promise.all(
      nonConvertableVideoFiles.map(file =>
        publisherChannel.sendToQueue(
          QueueName.FILE_MOVE,
          {
            src: file.path,
            dest: getFileOutputPath(file.name, TorrentPath.DOWNLOAD),
            torrentID: downloadedTorrent._id,
            fileSlug: file.slug,
          } as IMoveFilesMessageContent,
          { persistent: true }
        )
      )
    );
  }

  consumerChannel.ack(message);
  logger.info(`${downloadedTorrent.name} torrent downloaded now deleting torrent`);
  torrentInClient.destroy();
};

export const downloadTorrent =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  // eslint-disable-next-line sonarjs/cognitive-complexity
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    try {
      const addedTorrent: ITorrent = getMessageContent<ITorrent>(message);
      logger.info(`Received new torrent to download.. ${JSON.stringify(addedTorrent)}`);

      client.add(addedTorrent.magnet, { path: TorrentPath.TMP }, async torrent => {
        torrent.on('error', () => {
          logger.error(`Torrent error, ack msg: ' ${addedTorrent} `);
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
          torrent.on('metadata', async () => {
            if (torrent.progress === 1) {
              await handleCompletedTorrent(torrent, addedTorrent, publisherChannel, channel, message);
            }
          });
          torrent.on('done', async () => {
            await handleCompletedTorrent(torrent, SavedTorrent, publisherChannel, channel, message);
          });
        }
      });
    } catch (error) {
      logger.error(error);
      channel.ack(message);
    }
  };
