import { ChannelWrapper } from 'amqp-connection-manager';
import { Channel, ConsumeMessage } from 'amqplib';
import { TorrentPath, QueueName } from '../../@types';
import { IConvertVideoMessageContent, IDeleteFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { extractSubtitles, getFileOutputPath, getMessageContent } from '../../utils/misc';
import { updateFilePath } from '../../utils/query';
import { convertMKVtoMp4 } from '../../utils/videoConverter';

export const convertVideo =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;

    const file = getMessageContent<IConvertVideoMessageContent>(message);
    logger.info(`Received new video file to convert.. file:${file.name}`);
    const outputPath = getFileOutputPath(file.name, TorrentPath.DOWNLOAD);
    try {
      await extractSubtitles(file);
      const done = await convertMKVtoMp4(file.path, outputPath, file.slug, file.torrentID).catch(error => {
        logger.error(error);
      });
      if (done) {
        logger.info(`file converted successfully file: ${file.name}`);
        publisherChannel
          .sendToQueue(QueueName.FILE_DELETE, { src: file.path } as IDeleteFilesMessageContent)
          .then(() => {
            updateFilePath(file.torrentID, file.slug, outputPath).then(() => {
              channel.ack(message);
            });
          });
      } else {
        logger.error(`something went wrong while converting file: ${file.name}`);
        channel.ack(message);
      }
    } catch (error) {
      logger.error(`something went wrong while converting file: ${file.name} error: ${JSON.stringify(error)}`);
      channel.ack(message);
    }
  };
