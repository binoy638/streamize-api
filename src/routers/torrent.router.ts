import { Router } from 'express';
// eslint-disable-next-line import/no-cycle
import {
  addTorrent,
  deleteTorrentFromId,
  getAllTorrents,
  getTorrentInfoFromId,
} from '../controllers/torrent.controller';

const torrentRouter = Router();

torrentRouter.get('/', getAllTorrents);

torrentRouter.delete('/:id', deleteTorrentFromId);

torrentRouter.get('/:id', getTorrentInfoFromId);

torrentRouter.post('/', addTorrent);

export default torrentRouter;
