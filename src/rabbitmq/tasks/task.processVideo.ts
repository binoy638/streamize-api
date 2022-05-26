import { ChannelWrapper } from 'amqp-connection-manager';
import { Channel, ConsumeMessage } from 'amqplib';
import { QueueName, IVideo, VideoState } from '../../@types';
import { IConvertVideoMessageContent, IDeleteFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import VideoProcessor from '../../libs/videoProcessor';
import Utils from '../../utils';

//! temp solution
const isProcessed = (video: IVideo): boolean => {
  const status = video?.status;
  if (!status) return false;
  if (status === VideoState.DONE || status === VideoState.PROCESSING) return true;
  return false;
};

export const processVideo =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;

    const file = Utils.getMessageContent<IConvertVideoMessageContent>(message);

    logger.info(`Received new video file to convert.. file:${file.name}`);
    const video = await Utils.getVideoFile(file.slug, false);
    //* Checking if the video is already getting converted
    if (video && isProcessed(video) === false) {
      const videoProcessor = new VideoProcessor(file.torrentID, video);

      try {
        await videoProcessor.extractSubs();
        await videoProcessor.convertToHLS();

        logger.info(`file converted successfully file: ${file.name}`);
        const data: IDeleteFilesMessageContent = {
          src: file.path,
        };
        publisherChannel.sendToQueue(QueueName.FILE_DELETE, data, { persistent: true }).then(() => {
          channel.ack(message);
        });
      } catch (error) {
        logger.error(`something went wrong while converting file: ${file.name} error: ${JSON.stringify(error)}`);
        logger.info(`deleting file ${file.name}`);
        const data: IDeleteFilesMessageContent = {
          src: file.path,
        };
        publisherChannel.sendToQueue(QueueName.FILE_DELETE, data, { persistent: true });
        channel.ack(message);
      }
    } else {
      logger.error(`file already converted file: ${file.name}`);
    }
  };