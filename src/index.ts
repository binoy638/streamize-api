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
import logger from './config/logger';
import * as rabbitMQ from './rabbitmq';
import { TorrentModel } from './models/torrent.schema';

const PORT = 3000;

const app = express();

app.use(helmet());
app.use(morgan('common'));
app.use(cors());
app.use(express.json());

app.use('/torrent', torrentRouter);
app.use('/video', videoRouter);

app.listen(PORT, async () => {
  try {
    await connectMongo();
    if (process.env.NODE_ENV === 'development') {
      await fs.emptyDir(TorrentPath.DOWNLOAD);
      await fs.emptyDir(TorrentPath.TMP);
      await fs.emptyDir(TorrentPath.SUBTITLES);
      rabbitMQ.publisherChannel.ackAll();
      rabbitMQ.torrentChannel.ackAll();
      rabbitMQ.cpuIntensiveVideoProcessingChannel.ackAll();
      rabbitMQ.videoInspectionChannel.ackAll();
      rabbitMQ.fileManagerChannel.ackAll();
      rabbitMQ.nonCpuIntensiveVideoProcessingChannel.ackAll();
      await TorrentModel.deleteMany({});
    }

    logger.info(`Example app listening on port ${PORT}`);
  } catch (error) {
    logger.error(error);
  }
});

process.on('uncaughtException', error => {
  logger.error(error);
});

process.on('unhandledRejection', error => {
  logger.error(error);
});

app.on('error', error => logger.error(error));

app.use(notFoundHandler);
app.use(errorHandler);
