import { Request, Response } from 'express';
import boom from '@hapi/boom';
import 'express-async-errors';
import fs from 'fs-extra';
import client from '../config/webtorrent';
import {
  createTorrentWithMagnet,
  deleteTorrentByID,
  getTorrentBySlug,
  getAllTorrentsFromDB,
  // getTorrentByMagnet,
  clearTorrents,
} from '../utils/query';
import { fileManagerChannel, publisherChannel, torrentChannel, videoChannel } from '../rabbitmq';
import { QueueName, TorrentPath } from '../@types';
import { IDeleteFilesMessageContent } from '../@types/message';
import logger from '../config/logger';
import redisClient from '../config/redis';
import { getDataFromTorrent, getDataFromTorrentFile } from '../utils/torrentHelper';

export const getAllTorrents = async (req: Request, res: Response): Promise<void> => {
  const torrents = await getAllTorrentsFromDB();
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
};

export const getTorrentInfo = async (req: Request, res: Response): Promise<void> => {
  const { slug } = req.params;
  if (!slug) {
    throw boom.badRequest('slug is required');
  }
  const doc = await getTorrentBySlug(slug);
  if (doc) {
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
  } else {
    throw boom.notFound('torrent not found');
  }
};

export const deleteTorrent = async (req: Request, res: Response): Promise<void> => {
  const { slug } = req.params;
  if (!slug) {
    throw boom.badRequest('slug is required');
  }
  const doc = await getTorrentBySlug(slug);

  if (!doc) {
    throw boom.notFound('torrent not found');
  }

  const torrent = client.get(doc.infoHash);
  if (torrent) {
    torrent.destroy();
  }

  const Files = doc.files.map(file => {
    if (file.path) {
      publisherChannel.sendToQueue(QueueName.FILE_DELETE, { src: file.path } as IDeleteFilesMessageContent);
    }
    if (file.subtitles.length > 0) {
      const subs = file.subtitles.map(subtitle => {
        publisherChannel.sendToQueue(QueueName.FILE_DELETE, { src: subtitle.path } as IDeleteFilesMessageContent);
        return subtitle.title;
      });
      redisClient.del(`VIDEO_PATH:${file.slug}`).catch(error => logger.error(error));
      logger.info(`Deleting subtitles: ${subs}`);
    }
    return file.name;
  });

  logger.info(`Deleting torrent ${doc.slug}\nFiles: ${Files}`);

  await deleteTorrentByID(doc._id);
  res.send({ message: 'torrent deleted successfully' });
};

// {
//   path: path.join(__dirname, '../../public/torrents');
// }

export const addTorrent = async (req: Request, res: Response): Promise<void> => {
  const { magnet } = req.body;
  if (!magnet) {
    throw boom.badRequest('magnet link is required');
  }
  if (!magnet.startsWith('magnet:?xt=urn:btih:')) {
    throw boom.badRequest('invalid magnet link');
  }
  // const existInDB = await getTorrentByMagnet(magnet);
  // const existInWebTorrent = client.get(magnet);
  // if (existInDB || existInWebTorrent) {
  //   throw boom.badRequest('torrent already exist');
  // }

  const torrent = await createTorrentWithMagnet(magnet);

  publisherChannel
    .sendToQueue(QueueName.DOWNLOAD_TORRENT, torrent, { persistent: true, expiration: '86400000' })
    .then(() => {
      res.send({ message: 'torrent added successfully', torrent });
    })
    .catch(error => {
      logger.error(error);
      throw boom.internal('something went wrong');
    });
};

export const clearTemp = async (req: Request, res: Response): Promise<void> => {
  try {
    await fs.emptyDir(TorrentPath.TMP);
    res.send({ message: 'temp folder cleared successfully' });
  } catch (error) {
    logger.error(error);
    throw boom.internal('something went wrong');
  }
};

export const deleteAllTorrents = async (req: Request, res: Response): Promise<void> => {
  try {
    await fs.emptyDir(TorrentPath.DOWNLOAD);
    await fs.emptyDir(TorrentPath.TMP);
    publisherChannel.ackAll();
    torrentChannel.ackAll();
    videoChannel.ackAll();
    fileManagerChannel.ackAll();
    await clearTorrents();
    res.send({ message: 'All torrents deleted' });
  } catch (error) {
    logger.error(error);
    throw boom.internal('something went wrong while deleting torrents');
  }
};
