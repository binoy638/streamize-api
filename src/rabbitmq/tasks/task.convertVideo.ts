import { ChannelWrapper } from 'amqp-connection-manager';
import { Channel, ConsumeMessage } from 'amqplib';
import { TorrentPath, QueueName, IVideo } from '../../@types';
import { IConvertVideoMessageContent, IDeleteFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { extractSubtitles, getFileOutputPath, getMessageContent } from '../../utils/misc';
import { getVideoFile, updateFilePath } from '../../utils/query';
import { convertMKVtoMp4 } from '../../utils/videoConverter';

//! temp solution
const isProcessed = (video: IVideo): boolean => {
  const status = video?.convertStatus;
  if (!status) return false;
  if (status.state === 'done' || status.state === 'processing') return true;
  return false;
};

export const convertVideo =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;

    const file = getMessageContent<IConvertVideoMessageContent>(message);

    logger.info(`Received new video file to convert.. file:${file.name}`);
    const video = await getVideoFile(file.slug, false);
    //* Checking if the video is already getting converted
    if (video && isProcessed(video) === false) {
      const outputPath = getFileOutputPath(file.name, `${TorrentPath.DOWNLOAD}/${file.torrentSlug}/${file.slug}`);

      try {
        await extractSubtitles(file);
        const done = await convertMKVtoMp4(file.path, outputPath, file.slug, file.torrentID).catch(error => {
          logger.error(error);
        });
        if (done) {
          logger.info(`file converted successfully file: ${file.name}`);
          publisherChannel
            .sendToQueue(
              QueueName.FILE_DELETE,
              { src: file.path, torrentSlug: file.torrentSlug } as IDeleteFilesMessageContent,
              { persistent: true }
            )
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
    } else {
      logger.error(`file already converted file: ${file.name}`);
    }
  };
