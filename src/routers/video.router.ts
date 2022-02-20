import { Router } from 'express';
import { downloadVideo, getSubtitle, getVideo, getVideoInfo } from '../controllers/video.controller';

const videoRouter = Router();

videoRouter.get('/play/:videoSlug', getVideo);

videoRouter.get('/:videoSlug', getVideoInfo);

videoRouter.get('/subtitle/:filename', getSubtitle);

videoRouter.get('/download/:videoSlug', downloadVideo);

export default videoRouter;
