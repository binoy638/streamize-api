import { Router } from 'express';
import { getSubtitle, getVideo } from '../controllers/video.controller';

const videoRouter = Router();

videoRouter.get('/play/:videoSlug', getVideo);

videoRouter.get('/subtitle/:filename', getSubtitle);

export default videoRouter;
