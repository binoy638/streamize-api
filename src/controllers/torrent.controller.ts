import { Request, Response } from 'express';
import boom from '@hapi/boom';
import 'express-async-errors';
import client from '../config/webtorrent';
import {
  createTorrentWithMagnet,
  deleteTorrentByID,
  getTorrentBySlug,
  getAllTorrentsFromDB,
  getTorrentByMagnet,
} from '../utils/query';
import { publisherChannel } from '../rabbitmq';

export const getAllTorrents = async (req: Request, res: Response): Promise<void> => {
  const torrents = await getAllTorrentsFromDB();
  res.send({ torrents });
};

export const getTorrentInfo = async (req: Request, res: Response): Promise<void> => {
  const { slug } = req.params;
  if (!slug) {
    throw boom.badRequest('slug is required');
  }
  const doc = await getTorrentBySlug(slug);
  if (doc) {
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
  const existInDB = await getTorrentByMagnet(magnet);
  const existInWebTorrent = client.get(magnet);
  if (existInDB || existInWebTorrent) {
    throw boom.badRequest('torrent already exist');
  }

  const torrent = await createTorrentWithMagnet(magnet);

  publisherChannel
    .sendToQueue('download-torrent', torrent)
    .then(() => {
      res.send({ message: 'torrent added successfully', torrent });
    })
    .catch(error => {
      console.log(error);
      throw boom.internal('something went wrong');
    });
};
