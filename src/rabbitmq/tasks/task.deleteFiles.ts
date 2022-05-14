import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import { getMessageContent, isEmpty } from '../../utils/misc';
import { IDeleteFilesMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { TorrentPath } from '../../@types';

export const deleteFiles =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    const data = getMessageContent<IDeleteFilesMessageContent>(message);
    logger.info({ message: 'Received new video file to delete..', data });
    try {
      await fs.remove(data.src);
      logger.info({ message: 'file deleted successfully!', data });
      //* delete the directory if its empty
      const isDirEmpty = await isEmpty(`${TorrentPath.TMP}/${data.torrentSlug}`);
      if (isDirEmpty) {
        await fs.remove(`${TorrentPath.TMP}/${data.torrentSlug}`);
      }
      channel.ack(message);
    } catch (error) {
      logger.error({ message: 'something went wrong while deleting file', data, error });
      channel.ack(message);
    }
  };
