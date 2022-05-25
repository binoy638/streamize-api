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
    logger.info(`Received new video file to delete..\n ${JSON.stringify(data)} `);
    try {
      await fs.remove(data.src);
      logger.info(`file deleted successfully\n ${JSON.stringify(data)} `);
      channel.ack(message);
    } catch (error) {
      logger.info('something went wrong while deleting file');
      logger.error(error);
      channel.ack(message);
    }
  };
