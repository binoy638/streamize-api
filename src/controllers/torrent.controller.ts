import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import fs from 'fs-extra';
import client from '../config/webtorrent';
import * as rabbitMQ from '../rabbitmq';
import { ITorrent, QueueName, TorrentPath, TorrentState } from '../@types';
import logger from '../config/logger';
import { TorrentModel } from '../models/torrent.schema';
import Utils from '../utils';
import { UserModel } from '../models/user.schema';
import { ITorrentDownloadMessageContent } from '../@types/message';
import { MediaShareModel } from '../models/MediaShare';
import { UserVideoProgressModel } from '../models/userVideoProgress.schema';

/**
 *
 * @description fetch all avaialble torrents
 */

export const getall = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { currentUser } = req;
  try {
    const userDoc = await UserModel.findOne({ _id: currentUser.id })
      .populate<{ torrents: ITorrent[] }>('torrents')
      .lean();

    if (!userDoc) {
      next(boom.notFound('torrent not found'));
      return;
    }

    const { torrents } = userDoc;

    const torrentsWithDownloadInfo = torrents.map(torrent => {
      if (torrent.status === TorrentState.DOWNLOADING) {
        const torrentInClient = client.get(torrent.infoHash);
        if (torrentInClient) {
          return { ...torrent, downloadInfo: Utils.getDataFromTorrent(torrentInClient) };
        }
        return torrent;
      }
      return torrent;
    });

    //* calculate disk usage
    const diskSpace = await Utils.getUserDiskUsage(currentUser);

    res.send({ torrents: torrentsWithDownloadInfo, diskSpace });
  } catch (error) {
    logger.error(error);

    next(boom.internal('Internal server error'));
  }
};

/**
 *
 * @description fetch a torrent by slug
 */

export const get = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { slug } = req.params;
  const { currentUser } = req;
  try {
    const doc = await UserModel.findOne({ _id: currentUser.id }).populate<{ torrents: ITorrent[] }>('torrents').lean();

    if (!doc || doc.torrents.length === 0) {
      next(boom.notFound('torrent not found'));
      return;
    }

    const torrent = doc.torrents.find(tor => tor.slug === slug);

    if (!torrent) {
      next(boom.notFound('torrent not found'));
      return;
    }
    const torrentDoc = torrent;

    if (torrentDoc.status === TorrentState.DOWNLOADING) {
      const torrent = client.get(torrentDoc.infoHash);
      if (torrent) {
        const downloadInfo = Utils.getDataFromTorrent(torrent);

        const filesWithDownloadInfo = torrentDoc.files.map(docFile => {
          const file = torrent.files.find(file => file.name === docFile.name);
          if (file) {
            const downloadInfo = Utils.getDataFromTorrentFile(file);
            return { ...docFile, downloadInfo };
          }
          return docFile;
        });
        torrentDoc.downloadInfo = downloadInfo;
        torrentDoc.files = filesWithDownloadInfo;
      }
    }
    res.send(torrentDoc);
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

/**
 *
 * @description delete a torrent by slug
 */

export const del = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { slug } = req.params;
  const { currentUser } = req;
  try {
    const doc = await TorrentModel.findOne({ slug }).lean();

    if (!doc) {
      next(boom.notFound('torrent not found'));
      return;
    }

    const torrent = client.get(doc.magnet);
    if (torrent) {
      torrent.destroy();
    }

    rabbitMQ.publisherChannel.sendToQueue(QueueName.FILE_DELETE, { src: `${TorrentPath.TMP}/${doc.slug}` });

    //* put all files in queue to delete
    const Files = doc.files.map(file => {
      rabbitMQ.publisherChannel.sendToQueue(QueueName.FILE_DELETE, {
        src: `${TorrentPath.DOWNLOAD}/${file.slug}`,
      });
      rabbitMQ.publisherChannel.sendToQueue(QueueName.FILE_DELETE, {
        src: `${TorrentPath.SUBTITLES}/${file.slug}`,
      });
      return file.name;
    });

    logger.info(`Deleting torrent ${doc.slug}\nFiles: ${Files}`);

    const videos = doc.files.map(file => file.slug);

    await TorrentModel.deleteOne({ _id: doc._id });
    await UserModel.updateOne({ _id: currentUser.id }, { $pull: { torrents: doc._id } });
    await MediaShareModel.deleteMany({ torrent: doc._id });
    await UserVideoProgressModel.deleteMany({ video: { $in: videos } });
    res.send({ message: 'torrent deleted successfully' });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

/**
 *
 * @description add a torrent using magnet link
 */

export const add = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { magnet } = req.body;

  const { currentUser } = req;

  try {
    const existingTorrent = await TorrentModel.findOne({ magnet });
    if (existingTorrent) {
      //* check if the request user already have this torrent
      const existInCurrentUser = await UserModel.findOne({ _id: currentUser.id, torrents: existingTorrent._id });
      if (existInCurrentUser) {
        next(boom.badRequest('torrent already exist'));
        return;
      }
      //* add torrent to user
      await UserModel.updateOne({ _id: currentUser.id }, { $push: { torrents: existingTorrent._id } });
      res.send({ message: 'torrent added successfully', existingTorrent });
      return;
    }

    const doc = new TorrentModel({ magnet, status: TorrentState.ADDED });
    const torrent = await doc.save();

    await UserModel.updateOne({ _id: currentUser.id }, { $push: { torrents: torrent._id } });

    const data: ITorrentDownloadMessageContent = { currentUser, torrent };

    rabbitMQ.publisherChannel
      .sendToQueue(QueueName.DOWNLOAD_TORRENT, data, { persistent: true, expiration: '86400000' })
      .then(() => {
        res.send({ message: 'torrent added successfully', torrent });
      })
      .catch(error => {
        logger.error(error);
        next(boom.internal('Internal server error'));
      });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

/**
 *
 * @description clear temp download folder
 */

export const clearTemp = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    await fs.emptyDir(TorrentPath.TMP);
    res.send({ message: 'temp folder cleared successfully' });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

/**
 *
 * @description clear all torrents
 */

export const clearAll = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    await fs.emptyDir(TorrentPath.DOWNLOAD);
    await fs.emptyDir(TorrentPath.TMP);
    rabbitMQ.publisherChannel.ackAll();
    rabbitMQ.torrentChannel.ackAll();
    rabbitMQ.cpuIntensiveVideoProcessingChannel.ackAll();
    rabbitMQ.videoInspectionChannel.ackAll();
    rabbitMQ.fileManagerChannel.ackAll();
    rabbitMQ.nonCpuIntensiveVideoProcessingChannel.ackAll();
    await TorrentModel.deleteMany({});
    await UserVideoProgressModel.deleteMany({});
    await MediaShareModel.deleteMany({});
    res.send({ message: 'All torrents deleted' });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
