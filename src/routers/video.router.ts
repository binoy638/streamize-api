import { Router } from 'express';
import { getAllAvailableVideos, getVideo } from '../controllers/video.controller';

const videoRouter = Router();

videoRouter.get('/:id', getVideo);

videoRouter.get('/', getAllAvailableVideos);

export default videoRouter;
