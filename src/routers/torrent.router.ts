import { Router } from 'express';
import * as torrentController from '../controllers/torrent.controller';
import { getCurrentUser } from '../middlewares/currentUser.middleware';
import { isAdmin } from '../middlewares/isAdmin.middleware';
import { validator } from '../middlewares/validator.middleware';
import { torrentMagnetValidator, torrentSlugValidator } from '../validators/torrent.validator';

const torrentRouter = Router();

torrentRouter.get('/', getCurrentUser, torrentController.getall);

torrentRouter.delete('/:slug', getCurrentUser, validator(torrentSlugValidator), torrentController.del);

torrentRouter.delete('/clear/all', getCurrentUser, isAdmin, torrentController.clearAll);

torrentRouter.delete('/clear/temp', getCurrentUser, isAdmin, torrentController.clearTemp);

torrentRouter.get('/:slug', getCurrentUser, validator(torrentSlugValidator), torrentController.get);

torrentRouter.post('/', getCurrentUser, validator(torrentMagnetValidator), torrentController.add);

export default torrentRouter;
