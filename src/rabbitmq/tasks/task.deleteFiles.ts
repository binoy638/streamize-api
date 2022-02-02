import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent } from '../../utils/misc';
import { IDeleteFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';

export const deleteFiles =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const data = getMessageContent<IDeleteFilesMessageContent>(message);
    logger.info({ message: 'Received new video file to delete..', data });
    try {
      await fs.remove(data.src);
      logger.info({ message: 'file deleted successfully!', data });
      channel.ack(message);
    } catch (error) {
      logger.error({ message: 'something went wrong while deleting file', data, error });
      channel.ack(message);
    }
  };
