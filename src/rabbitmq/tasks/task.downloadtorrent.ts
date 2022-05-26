/* eslint-disable unicorn/consistent-destructuring */
import { Channel, ChannelWrapper } from 'amqp-connection-manager';
import { ConsumeMessage } from 'amqplib';
import { Torrent } from 'webtorrent';
import { ITorrent, TorrentPath, IVideo, QueueName, VideoState, TorrentState } from '../../@types';
import { IProcessVideoMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import client from '../../config/webtorrent';
import { TorrentModel } from '../../models/torrent.schema';
import Utils from '../../utils';

const handleCompletedTorrent = async (
  torrentInClient: Torrent,
  downloadedTorrent: ITorrent,
  publisherChannel: ChannelWrapper,
  consumerChannel: Channel,
  message: ConsumeMessage
) => {
  //* sending all video files to process-video queue
  downloadedTorrent.files.map(file =>
    publisherChannel.sendToQueue(
      QueueName.INSPECT_VIDEO,
      {
        torrentID: downloadedTorrent._id,
        torrentSlug: downloadedTorrent.slug,
        ...file,
      } as IProcessVideoMessageContent,
      { persistent: true }
    )
  );
  await TorrentModel.updateOne({ _id: downloadedTorrent._id }, { status: TorrentState.QUEUED });

  consumerChannel.ack(message);
  logger.info(`${downloadedTorrent.name} torrent downloaded now deleting torrent`);
  torrentInClient.destroy();
};

export const downloadTorrent =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  // eslint-disable-next-line sonarjs/cognitive-complexity
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const addedTorrent: ITorrent = Utils.getMessageContent<ITorrent>(message);
    logger.info(`Received new torrent to download.. ${JSON.stringify(addedTorrent)}`);
    try {
      client.add(addedTorrent.magnet, { path: `${TorrentPath.TMP}/${addedTorrent.slug}` }, async torrent => {
        torrent.on('error', error => {
          logger.error(`Torrent error: ' ${addedTorrent} `);
          logger.error(error);
          channel.ack(message);
        });

        const videofiles = torrent.files
          .map(file => {
            const ext = file.name.split('.').pop() || '';
            return {
              name: file.name,
              path: file.path,
              size: file.length,
              ext,
              status: VideoState.DOWNLOADING,
            } as IVideo;
          })
          //* filtering torrent files based on media extensions
          .filter(file => Utils.supportedMediaExtension.has(file.ext));

        //* If no supported media files found remove the torrent
        if (videofiles.length === 0) {
          torrent.destroy({ destroyStore: true });
          logger.info({ message: 'no video files found, deleting torrent', addedTorrent });
          // eslint-disable-next-line no-underscore-dangle
          await TorrentModel.updateOne({ _id: addedTorrent._id }, { status: TorrentState.ERROR, isMedia: false });
          channel.ack(message);
          return;
        }

        const { name, infoHash, length: size } = torrent;

        const SavedTorrent = await TorrentModel.findOneAndUpdate(
          { _id: addedTorrent._id },
          {
            name,
            infoHash,
            size,
            files: videofiles,
            status: TorrentState.DOWNLOADING,
          },
          { new: true, lean: true }
        );

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
      });
    } catch (error) {
      logger.error(error);
      await Utils.updateTorrentStatus(addedTorrent._id, TorrentState.ERROR);
      channel.ack(message);
    }
  };
