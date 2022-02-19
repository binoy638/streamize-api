import { Router } from 'express';
// eslint-disable-next-line import/no-cycle
import {
  addTorrent,
  clearTemp,
  deleteAllTorrents,
  deleteTorrent,
  getAllTorrents,
  getTorrentInfo,
} from '../controllers/torrent.controller';

const torrentRouter = Router();

torrentRouter.get('/', getAllTorrents);

torrentRouter.delete('/:slug', deleteTorrent);

torrentRouter.delete('/all', deleteAllTorrents);

torrentRouter.delete('/temp', clearTemp);

torrentRouter.get('/:slug', getTorrentInfo);

torrentRouter.post('/', addTorrent);

export default torrentRouter;
