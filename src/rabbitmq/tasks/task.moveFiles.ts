import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent } from '../../utils/misc';
import { IMoveFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { updateFilePath, updateTorrentFileStatus } from '../../utils/query';

export const moveFiles =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const file = getMessageContent<IMoveFilesMessageContent>(message);
    logger.info({ message: 'Received new video file to move..', file });
    try {
      await fs.move(file.src, file.dest);
      await updateFilePath(file.torrentID, file.fileSlug, file.dest);
      await updateTorrentFileStatus(file.torrentID, file.fileSlug, 'done');
      logger.info(`file moved successfully: ${JSON.stringify(file)}`);
      channel.ack(message);
    } catch (error) {
      logger.error(error);
      channel.ack(message);
    }
  };
