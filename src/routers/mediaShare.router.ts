import { Router } from 'express';
import * as Yup from 'yup';
import * as mediaShareController from '../controllers/mediaShareController';
import { streamVideo } from '../controllers/video.controller';
import { getCurrentUser } from '../middlewares/currentUser.middleware';
import { validator } from '../middlewares/validator.middleware';

const mediaShareRouter = Router();

mediaShareRouter.get(
  '/:slug',
  validator({
    params: Yup.object().shape({
      slug: Yup.string().min(5).max(5).required(),
    }),
  }),
  mediaShareController.getPlaylist
);

mediaShareRouter.get(
  '/stream/:slug/:videoSlug/:filename',
  validator({
    params: Yup.object().shape({
      slug: Yup.string().min(5).max(5).required(),
      videoSlug: Yup.string().min(5).max(5).required(),
      filename: Yup.string().required(),
    }),
  }),
  streamVideo
);

mediaShareRouter.post(
  '/create',
  validator({
    body: Yup.object().shape({
      torrent: Yup.string().required(),
      mediaId: Yup.string().required(),
      isTorrent: Yup.boolean().required(),
      //   expiresIn: Yup.date().required(),
    }),
  }),
  getCurrentUser,
  mediaShareController.create
);

export default mediaShareRouter;
