import { Router } from 'express';
import { getSubtitle, getVideo, getVideoInfo } from '../controllers/video.controller';

const videoRouter = Router();

videoRouter.get('/play/:videoSlug', getVideo);

videoRouter.get('/:videoSlug', getVideoInfo);

videoRouter.get('/subtitle/:filename', getSubtitle);

export default videoRouter;
