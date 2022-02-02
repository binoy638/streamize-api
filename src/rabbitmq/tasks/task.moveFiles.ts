import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent } from '../../utils/misc';
import { IMoveFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';

export const moveFiles =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const file = getMessageContent<IMoveFilesMessageContent>(message);
    logger.info({ message: 'Received new video file to move..', file });
    try {
      await fs.move(file.src, file.dest);
      logger.info({ message: 'file moved successfully!', file });
      channel.ack(message);
    } catch (error) {
      logger.error({ message: 'something went wrong while moving file', file, error });
      channel.ack(message);
    }
  };
