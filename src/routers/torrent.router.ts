import { Router } from 'express';
import * as torrentController from '../controllers/torrent.controller';

const torrentRouter = Router();

torrentRouter.get('/', torrentController.getall);

torrentRouter.delete('/:slug', torrentController.del);

torrentRouter.delete('/clear/all', torrentController.clearAll);

torrentRouter.delete('/clear/temp', torrentController.clearTemp);

torrentRouter.get('/:slug', torrentController.get);

torrentRouter.post('/', torrentController.add);

export default torrentRouter;
