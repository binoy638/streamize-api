import { Router } from 'express';
import { getTorrentVideos, getVideo } from '../controllers/video.controller';

const videoRouter = Router();

videoRouter.get('/play/:torrentSlug/:videoSlug', getVideo);

videoRouter.get('/:slug', getTorrentVideos);

export default videoRouter;
