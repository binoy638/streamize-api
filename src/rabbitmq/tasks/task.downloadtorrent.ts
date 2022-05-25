/* eslint-disable unicorn/consistent-destructuring */
import { Channel, ChannelWrapper } from 'amqp-connection-manager';
import { ConsumeMessage } from 'amqplib';
import { Torrent } from 'webtorrent';
import { ITorrent, TorrentPath, IVideo, QueueName } from '../../@types';
import { IConvertVideoMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import client from '../../config/webtorrent';
import { TorrentModel } from '../../models/torrent.schema';
import { allowedExt, convertableExt, getMessageContent } from '../../utils/misc';
import { updateTorrentInfo } from '../../utils/query';

const handleCompletedTorrent = async (
  torrentInClient: Torrent,
  downloadedTorrent: ITorrent,
  publisherChannel: ChannelWrapper,
  consumerChannel: Channel,
  message: ConsumeMessage
) => {
  await TorrentModel.updateOne({ _id: downloadedTorrent._id }, { status: 'waiting' });

  //* sending all video files to process-video queue
  downloadedTorrent.files.map(file =>
    publisherChannel.sendToQueue(
      QueueName.PROCESS_VIDEO,
      {
        torrentID: downloadedTorrent._id,
        torrentSlug: downloadedTorrent.slug,
        ...file,
      } as IConvertVideoMessageContent,
      { persistent: true }
    )
  );

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

      client.add(addedTorrent.magnet, { path: `${TorrentPath.TMP}/${addedTorrent.slug}` }, async torrent => {
        torrent.on('error', error => {
          logger.error(`Torrent error: ' ${addedTorrent} `);
          logger.error(error);
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
          if (torrent.progress === 1) {
            logger.info(`torrent already downloaded, inside ready`);
            await handleCompletedTorrent(torrent, SavedTorrent, publisherChannel, channel, message);
          }
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
