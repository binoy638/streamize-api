import { Router } from 'express';
import * as torrentController from '../controllers/torrent.controller';
import { validator } from '../middlewares/validator.middleware';
import { torrentMagnetValidator, torrentSlugValidator } from '../validators/torrent.validator';

const torrentRouter = Router();

torrentRouter.get('/', torrentController.getall);

torrentRouter.delete('/:slug', validator(torrentSlugValidator), torrentController.del);

torrentRouter.delete('/clear/all', torrentController.clearAll);

torrentRouter.delete('/clear/temp', torrentController.clearTemp);

torrentRouter.get('/:slug', validator(torrentSlugValidator), torrentController.get);

torrentRouter.post('/', validator(torrentMagnetValidator), torrentController.add);

export default torrentRouter;
