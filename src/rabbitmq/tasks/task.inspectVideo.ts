//* task to identify where a video need to be transcoded

import { Channel, ChannelWrapper } from 'amqp-connection-manager';
import { ConsumeMessage } from 'amqplib';
import { IProcessVideoMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import Utils from '../../utils';
import VideoProcessor from '../../libs/videoProcessor';
import { QueueName, VideoState } from '../../@types';

export const inspectVideo =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const data = Utils.getMessageContent<IProcessVideoMessageContent>(message);
    logger.info(`Received new video file to inspect : ${data.name} `);
    try {
      const videoProcessor = new VideoProcessor(data.torrentID, data);
      const { audioCodec, videoCodec } = await videoProcessor.getCompatibleCodecs();
      if (audioCodec === 'copy' && videoCodec === 'copy') {
        logger.info(`Video doesn't need transcoding: ${data.name}`);
        publisherChannel.sendToQueue(QueueName.PROCESS_VIDEO_NON_CPU_INTENSIVE, data, { persistent: true });
        channel.ack(message);
        return;
      }
      logger.info(`Video need transcoding: ${data.name}`);
      publisherChannel.sendToQueue(QueueName.PROCESS_VIDEO_CPU_INTENSIVE, data, { persistent: true });
      channel.ack(message);
    } catch (error) {
      logger.info('something went wrong while inspecting file');
      await Utils.updateVideoFileStatus(data.torrentID, data.slug, VideoState.ERROR);
      logger.error(error);
      channel.ack(message);
    }
  };
