import { ConsumeMessage } from 'amqplib';
import { IVideo } from '../@types';
import logger from '../config/logger';
import { TorrentModel } from '../models/torrent.schema';

/* eslint-disable unicorn/no-static-only-class */
class Utils {
  static readonly supportedMediaExtension = new Set(['mp4', 'mkv', 'avi', 'mov', 'flv', 'mp3']);

  static getMessageContent<T>(message: ConsumeMessage): T {
    if (typeof message.content === 'string') {
      return message.content as T;
    }
    return JSON.parse(message.content.toString()) as T;
  }

  static getVideoFile = async (videoSlug: string, downloaded: boolean): Promise<IVideo | null | undefined> => {
    try {
      const doc = await TorrentModel.findOne({ 'files.slug': videoSlug }).lean(true);
      // eslint-disable-next-line unicorn/no-null
      if (!doc) return null;
      if (downloaded === true) {
        return doc.files.find(file => file.slug === videoSlug && file.status === 'done');
      }
      return doc.files.find(file => file.slug === videoSlug);
    } catch (error) {
      logger.error(error);
      throw new Error('something went wrong while fetching video file');
    }
  };
}

export default Utils;
