import { Router } from 'express';
import multer from 'multer';
import * as videoController from '../controllers/video.controller';
import { getCurrentUser } from '../middlewares/currentUser.middleware';
import { validator } from '../middlewares/validator.middleware';
import {
  streamVideoValidator,
  videoProgressGetValidator,
  videoProgressPostValidator,
} from '../validators/video.validator';

const videoRouter = Router();

const upload = multer({ dest: 'uploads/' });

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

videoRouter.get(
  '/progress/:videoSlug',
  getCurrentUser,
  validator(videoProgressGetValidator),
  videoController.getUserVideoProgress
);
videoRouter.post('/subtitle', upload.single('file'), videoController.addSubtitle);

export default videoRouter;
