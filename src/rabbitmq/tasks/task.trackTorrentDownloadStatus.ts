/* eslint-disable sonarjs/no-identical-functions */
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent } from '../../utils/misc';
import { ITorrentDownloadStatusMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import client from '../../config/webtorrent';
import { getDataFromTorrent } from '../../utils/torrentHelper';
import { updateTorrentDownloadInfo } from '../../utils/query';

export const trackDownload =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const data = getMessageContent<ITorrentDownloadStatusMessageContent>(message);
    logger.info(`Received new file to track download status: ${JSON.stringify(data)}`);

    try {
      const timer = setInterval(() => {
        const torrent = client.get(data.torrentInfoHash);
        if (!torrent) {
          clearInterval(timer);
          channel.ack(message);
          logger.debug('torrent not found from setInterval');
          updateTorrentDownloadInfo(data.torrentID, {
            downloadSpeed: 0,
            uploadSpeed: 0,
            progress: 1,
            timeRemaining: 0,
            paused: false,
            completed: true,
          });
          return;
        }
        if (torrent.done) {
          updateTorrentDownloadInfo(data.torrentID, {
            downloadSpeed: 0,
            uploadSpeed: 0,
            progress: 1,
            timeRemaining: 0,
            paused: false,
            completed: true,
          })
            .then(() => {
              channel.ack(message);
              clearInterval(timer);
              logger.info('torrent download complete');
            })
            .catch(error => {
              logger.error(error);
              channel.ack(message);
              clearInterval(timer);
            });
        }
        const downloadInfo = getDataFromTorrent(torrent);
        logger.debug(`torrent download status received: ${JSON.stringify(downloadInfo)}`);
        updateTorrentDownloadInfo(data.torrentID, downloadInfo)
          .then(() => {
            if (downloadInfo.completed) {
              channel.ack(message);
              clearInterval(timer);
              logger.info('torrent download complete');
            }
          })
          .catch(error => {
            logger.error(error);
            channel.ack(message);
            clearInterval(timer);
          });
      }, 5000);
    } catch (error) {
      logger.error(error);
      channel.ack(message);
    }
  };
