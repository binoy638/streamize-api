import { Router } from 'express';
import { getSubtitle, streamVideo, getVideoInfo, getPreview } from '../controllers/video.controller';
import { validator } from '../middlewares/validator.middleware';
import { streamVideoValidator } from '../validators/video.validator';

const videoRouter = Router();

videoRouter.get('/stream/:videoSlug/:filename', validator(streamVideoValidator), streamVideo);

videoRouter.get('/:videoSlug', getVideoInfo);

videoRouter.get('/subtitle/:videoSlug/:filename', validator(streamVideoValidator), getSubtitle);
videoRouter.get('/preview/:videoSlug/:filename', validator(streamVideoValidator), getPreview);

export default videoRouter;
