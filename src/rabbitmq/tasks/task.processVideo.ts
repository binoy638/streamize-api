import { ChannelWrapper } from 'amqp-connection-manager';
import { Channel, ConsumeMessage } from 'amqplib';
import { QueueName, IVideo, VideoState } from '../../@types';
import {
  IProcessVideoMessageContent,
  IDeleteFilesMessageContent,
  ISpriteGenerationMessageContent,
} from '../../@types/message';
import logger from '../../config/logger';
import VideoProcessor from '../../libs/videoProcessor';
import { TorrentModel } from '../../models/torrent.schema';
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

    const file = Utils.getMessageContent<IProcessVideoMessageContent>(message);

    logger.info(`Received new file to convert.. file:${file.name}`);
    const video = await Utils.getVideoFile(file.slug, false);

    if (!video) {
      logger.error(`Video not found ${file.slug}`);
      channel.ack(message);
      return;
    }
    //* Checking if the video is already getting converted
    if (video && isProcessed(video) === false) {
      const videoProcessor = new VideoProcessor(file.torrentID, video);

      try {
        await videoProcessor.extractSubs();
        const inputPath = await videoProcessor.convertToHLS();

        logger.info(`file converted successfully file: ${file.name}`);
        await TorrentModel.updateOne(
          { _id: file.torrentID, 'files.slug': file.slug },
          { $set: { 'files.$.status': VideoState.DONE } }
        );
        const deleteFiledata: IDeleteFilesMessageContent = {
          src: file.path,
        };
        const generateSpriteData: ISpriteGenerationMessageContent = {
          ...file,
          inputPath,
        };
        await publisherChannel.sendToQueue(QueueName.FILE_DELETE, deleteFiledata, { persistent: true });
        await publisherChannel.sendToQueue(QueueName.GENERATE_SPRITE, generateSpriteData, { persistent: true });
        channel.ack(message);
      } catch (error) {
        logger.error(`something went wrong while converting file: ${file.name} error: ${JSON.stringify(error)}`);
        await Utils.updateVideoFileStatus(file.torrentID, file.slug, VideoState.ERROR);
        channel.ack(message);
      }
    } else {
      logger.error(`file already converted file: ${file.name}`);
    }
  };
