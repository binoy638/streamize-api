import { Router } from 'express';
import {
  addTorrent,
  deleteTorrentFromId,
  getAllTorrents,
  getTorrentConvertFromId,
  getTorrentInfoFromId,
} from '../controllers/torrent.controller';

const torrentRouter = Router();

torrentRouter.get('/', getAllTorrents);

torrentRouter.delete('/:id', deleteTorrentFromId);

torrentRouter.get('/:id', getTorrentInfoFromId);

torrentRouter.get('/convert/:id', getTorrentConvertFromId);

torrentRouter.post('/', addTorrent);

export default torrentRouter;
