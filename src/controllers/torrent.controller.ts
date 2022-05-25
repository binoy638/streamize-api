import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import fs from 'fs-extra';
import client from '../config/webtorrent';
import { fileManagerChannel, publisherChannel, torrentChannel, videoChannel } from '../rabbitmq';
import { QueueName, TorrentPath } from '../@types';
import logger from '../config/logger';
import { getDataFromTorrent, getDataFromTorrentFile } from '../utils/torrentHelper';
import { TorrentModel } from '../models/torrent.schema';

/**
 *
 * @description fetch all avaialble torrents
 */

export const getall = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    const torrents = await TorrentModel.getTorrents();

    const torrentsWithDownloadInfo = torrents.map(torrent => {
      if (torrent.status === 'downloading') {
        const torrentInClient = client.get(torrent.infoHash);
        if (torrentInClient) {
          return { ...torrent, downloadInfo: getDataFromTorrent(torrentInClient) };
        }
        return torrent;
      }
      return torrent;
    });

    res.send({ torrents: torrentsWithDownloadInfo });
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

  try {
    const doc = await TorrentModel.findOne({ slug }).lean();

    if (!doc) {
      next(boom.notFound('torrent not found'));
      return;
    }

    if (doc.status === 'downloading') {
      const torrent = client.get(doc.infoHash);
      if (torrent) {
        const downloadInfo = getDataFromTorrent(torrent);

        const filesWithDownloadInfo = doc.files.map(docFile => {
          const file = torrent.files.find(file => file.name === docFile.name);
          if (file) {
            const downloadInfo = getDataFromTorrentFile(file);
            return { ...docFile, downloadInfo };
          }
          return docFile;
        });
        doc.downloadInfo = downloadInfo;
        doc.files = filesWithDownloadInfo;
      }
    }

    res.send(doc);
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

  try {
    const doc = await TorrentModel.findOne({ slug }).lean();

    if (!doc) {
      next(boom.notFound('torrent not found'));
      return;
    }

    const torrent = client.get(doc.infoHash);
    if (torrent) {
      torrent.destroy();
    }

    publisherChannel.sendToQueue(QueueName.FILE_DELETE, { src: `${TorrentPath.TMP}/${doc.slug}` });

    //* put all files in queue to delete
    const Files = doc.files.map(file => {
      publisherChannel.sendToQueue(QueueName.FILE_DELETE, { src: `${TorrentPath.DOWNLOAD}/${file.slug}` });
      publisherChannel.sendToQueue(QueueName.FILE_DELETE, { src: `${TorrentPath.SUBTITLES}/${file.slug}` });
      return file.name;
    });

    logger.info(`Deleting torrent ${doc.slug}\nFiles: ${Files}`);

    await TorrentModel.deleteOne({ _id: doc._id });
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

  try {
    const exists = await TorrentModel.findOne({ magnet });
    if (exists) {
      next(boom.badRequest('torrent already exist'));
      return;
    }

    const doc = new TorrentModel({ magnet });
    const torrent = await doc.save();

    publisherChannel
      .sendToQueue(QueueName.DOWNLOAD_TORRENT, torrent, { persistent: true, expiration: '86400000' })
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
    publisherChannel.ackAll();
    torrentChannel.ackAll();
    videoChannel.ackAll();
    fileManagerChannel.ackAll();
    await TorrentModel.deleteMany({});
    res.send({ message: 'All torrents deleted' });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
