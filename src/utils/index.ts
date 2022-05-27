import { ConsumeMessage } from 'amqplib';
import { Torrent, TorrentFile } from 'webtorrent';
import checkDiskSpace from 'check-disk-space';
import { IVideo, IDownloadInfo, IFileDownloadInfo, VideoState, TorrentState, TorrentPath } from '../@types';
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

  static getDataFromTorrent = (torrent: Torrent): IDownloadInfo => {
    return {
      downloadSpeed: torrent.downloadSpeed,
      uploadSpeed: torrent.uploadSpeed,
      progress: torrent.progress,
      timeRemaining: torrent.timeRemaining,
      paused: torrent.paused,
      completed: torrent.done,
    };
  };

  static getDataFromTorrentFile = (file: TorrentFile): IFileDownloadInfo => {
    return {
      downloaded: file.downloaded,
      progress: file.progress,
    };
  };

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

  static updateVideoFileStatus = async (torrentID: string, videoSlug: string, status: VideoState): Promise<void> => {
    try {
      await TorrentModel.updateOne({ _id: torrentID, 'files.slug': videoSlug }, { $set: { 'files.$.status': status } });
    } catch (error) {
      logger.error(error);
      throw new Error('something went wrong while updating video file status');
    }
  };

  static updateTorrentStatus = async (torrentID: string, status: TorrentState): Promise<void> => {
    try {
      await TorrentModel.updateOne({ _id: torrentID }, { status });
    } catch (error) {
      logger.error(error);
    }
  };

  static getDiskSpace = async (): Promise<{ size: number; free: number }> => {
    try {
      const diskSpace = await checkDiskSpace(TorrentPath.DOWNLOAD);
      // eslint-disable-next-line unicorn/numeric-separators-style
      return { size: diskSpace.size - 2000000000, free: diskSpace.free - 2000000000 };
    } catch (error) {
      logger.error(error);
      return { size: 0, free: 0 };
    }
  };
}

export default Utils;
