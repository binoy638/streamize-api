import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent } from '../../utils/misc';
import { IMoveFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { updateFilePath } from '../../utils/query';

export const moveFiles =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const file = getMessageContent<IMoveFilesMessageContent>(message);
    logger.info({ message: 'Received new video file to move..', file });
    try {
      await fs.move(file.src, file.dest);
      updateFilePath(file.torrentID, file.fileSlug, file.dest).then(() => {
        logger.info({ message: 'file moved successfully!', file });
        channel.ack(message);
      });
    } catch (error) {
      logger.error({ message: 'something went wrong while moving file', file, error });
      channel.ack(message);
    }
  };
