/* eslint-disable unicorn/no-array-reduce */
import { ConsumeMessage } from 'amqplib';
import { Torrent, TorrentFile } from 'webtorrent';
import checkDiskSpace from 'check-disk-space';
import {
  IVideo,
  IDownloadInfo,
  IFileDownloadInfo,
  VideoState,
  TorrentState,
  TorrentPath,
  UserPayload,
  ITorrent,
} from '../@types';
import logger from '../config/logger';
import { TorrentModel } from '../models/torrent.schema';
import { UserModel } from '../models/user.schema';

/* eslint-disable unicorn/no-static-only-class */
class Utils {
  static readonly supportedMediaExtension = new Set(['mp4', 'mkv', 'avi', 'mov', 'flv', 'mp3', 'wmv']);

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
      timeRemaining: torrent.timeRemaining ? torrent.timeRemaining : 0,
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
        return doc.files.find(file => file.slug === videoSlug && file.status === VideoState.DONE);
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

  static getUserDiskUsage = async (user: UserPayload): Promise<{ size: number; free: number }> => {
    try {
      const doc = await UserModel.findOne({ _id: user.id }).populate<{ torrents: ITorrent[] }>('torrents').lean();
      if (!doc) throw new Error('user not found');
      const usedSpace = doc.torrents.reduce((acc, torrent) => {
        if (torrent.status === TorrentState.DONE) {
          return acc + torrent.size;
        }
        return torrent.files.reduce((acc, file) => {
          return acc + file.size;
        }, 0);
      }, 0);

      if (user.isAdmin) {
        const users = await UserModel.find({ isAdmin: false });

        const spaceAllocatedToOthers = users.reduce((acc, user) => {
          return acc + user.allocatedMemory;
        }, 0);

        // eslint-disable-next-line sonarjs/prefer-immediate-return
        const diskSpace = await Utils.getDiskSpace();
        const adminTotalSpace = diskSpace.size - spaceAllocatedToOthers;
        return { size: adminTotalSpace, free: adminTotalSpace - usedSpace };
      }

      return { size: user.allocatedMemory, free: user.allocatedMemory - usedSpace };
    } catch (error) {
      logger.error(error);
      throw new Error('something went wrong while fetching user disk usage');
    }
  };
}

export default Utils;
