import express from 'express';
import morgan from 'morgan';
import cors from 'cors';
import helmet from 'helmet';
import fs from 'fs-extra';
import torrentRouter from './routers/torrent.router';
import videoRouter from './routers/video.router';
import notFoundHandler from './middlewares/notFoundHandler';
import errorHandler from './middlewares/errorHandler';
import connectMongo from './config/mongo';
import { TorrentPath } from './@types';
import { clearTorrents } from './utils/query';
import logger from './config/logger';
import { fileManagerChannel, publisherChannel, torrentChannel, videoChannel } from './rabbitmq';
import redisClient from './config/redis';

// eslint-disable-next-line import/no-mutable-exports

const PORT = 3000;

const app = express();

app.use(helmet());
app.use(morgan('common'));
app.use(cors());
app.use(express.json());

app.use('/torrent', torrentRouter);
app.use('/video', videoRouter);

// app.get('/test', async (req, res) => {

// });

app.listen(PORT, async () => {
  try {
    await fs.emptyDir(TorrentPath.DOWNLOAD);
    await fs.emptyDir(TorrentPath.TMP);
    await fs.emptyDir(TorrentPath.SUBTITLES);
    await connectMongo();
    await redisClient.connect();
    publisherChannel.ackAll();
    torrentChannel.ackAll();
    videoChannel.ackAll();
    fileManagerChannel.ackAll();
    await clearTorrents();

    logger.info(`Example app listening on port ${PORT}`);
  } catch (error) {
    logger.error(error);
  }
});

app.on('error', error => logger.error(error));

app.use(notFoundHandler);
app.use(errorHandler);
