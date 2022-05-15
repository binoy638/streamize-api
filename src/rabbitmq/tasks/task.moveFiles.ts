import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent, isDirEmpty } from '../../utils/misc';
import { IMoveFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { updateFilePath, updateTorrentFileStatus } from '../../utils/query';
import { TorrentPath } from '../../@types';

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
      //* delete the directory if its empty
      const isEmpty = await isDirEmpty(`${TorrentPath.TMP}/${file.torrentSlug}`);

      if (isEmpty) {
        await fs.remove(`${TorrentPath.TMP}/${file.torrentSlug}`);
      }
      channel.ack(message);
    } catch (error) {
      logger.error(error);
      channel.ack(message);
    }
  };
