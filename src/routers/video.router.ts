import { Router } from 'express';
import * as videoController from '../controllers/video.controller';
import { getCurrentUser } from '../middlewares/currentUser.middleware';
import { validator } from '../middlewares/validator.middleware';
import {
  streamVideoValidator,
  videoProgressGetValidator,
  videoProgressPostValidator,
} from '../validators/video.validator';

const videoRouter = Router();

videoRouter.get(
  '/stream/:videoSlug/:filename',
  getCurrentUser,
  validator(streamVideoValidator),
  videoController.streamVideo
);
videoRouter.get(
  '/media-share/:id/stream/:videoSlug/:filename',
  validator(streamVideoValidator),
  videoController.streamVideo
);

videoRouter.get('/:videoSlug', getCurrentUser, videoController.getVideoInfo);

videoRouter.get('/subtitle/:videoSlug/:filename', validator(streamVideoValidator), videoController.getSubtitle);

videoRouter.get('/preview/:videoSlug/:filename', validator(streamVideoValidator), videoController.getPreview);

videoRouter.post(
  '/progress/:videoSlug',
  getCurrentUser,
  validator(videoProgressPostValidator),
  videoController.saveUserVideoProgress
);

videoRouter.get(
  '/progress/:videoSlug',
  getCurrentUser,
  validator(videoProgressGetValidator),
  videoController.getUserVideoProgress
);

export default videoRouter;
