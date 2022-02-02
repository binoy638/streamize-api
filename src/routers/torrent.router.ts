import { Router } from 'express';
// eslint-disable-next-line import/no-cycle
import { addTorrent, deleteTorrent, getAllTorrents, getTorrentInfo } from '../controllers/torrent.controller';

const torrentRouter = Router();

torrentRouter.get('/', getAllTorrents);

torrentRouter.delete('/:slug', deleteTorrent);

torrentRouter.get('/:slug', getTorrentInfo);

torrentRouter.post('/', addTorrent);

export default torrentRouter;
