/* eslint-disable @typescript-eslint/no-non-null-assertion */
import express from 'express';
import morgan from 'morgan';
import cors from 'cors';
import cookieSession from 'cookie-session';
import helmet from 'helmet';
import fs from 'fs-extra';
import dotenv from 'dotenv';
import torrentRouter from './routers/torrent.router';
import videoRouter from './routers/video.router';
import notFoundHandler from './middlewares/notFoundHandler';
import errorHandler from './middlewares/errorHandler';
import connectMongo from './config/mongo';
import { TorrentPath } from './@types';
import logger from './config/logger';
import * as rabbitMQ from './rabbitmq';
import { TorrentModel } from './models/torrent.schema';
import authRouter from './routers/auth.router';
import { UserModel } from './models/user.schema';
import mediaShareRouter from './routers/mediaShare.router';

dotenv.config();

const PORT = 3000;

const app = express();

app.use(helmet());
if (process.env.NODE_ENV === 'development') {
  app.use(morgan('common'));
}
app.set('trust proxy', true);
app.use(
  cookieSession({
    secret: process.env.COOKIE_SECRET!,
    keys: [process.env.COOKIE_KEY!],
    maxAge: 24 * 60 * 60 * 1000 * 7,
    // sameSite: 'none',

    //* https only cookies
    secure: process.env.NODE_ENV !== 'development',
  })
);

app.use(cors({ origin: true, credentials: true }));
app.use(express.json());

app.use('/torrent', torrentRouter);
app.use('/video', videoRouter);
app.use('/auth', authRouter);
app.use('/share', mediaShareRouter);

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
  try {
    if (!process.env.ADMIN_USER && !process.env.ADMIN_PASSWORD && !process.env.JWT_SECRET)
      throw new Error('Admin credentials not set');

    const existingAdmin = await UserModel.findOne({ username: process.env.ADMIN_USER });
    if (!existingAdmin) {
      await UserModel.create({ username: process.env.ADMIN_USER, password: process.env.ADMIN_PASSWORD, isAdmin: true });
    }
    // await UserModel.updateOne({ username: process.env.ADMIN_USER }, { $set: { torrents: [] } });
    // const Torrents = await TorrentModel.getTorrents();
    // const torrentIds = Torrents.map(torrent => torrent._id);
    // await UserModel.updateOne({ username: process.env.ADMIN_USER }, { $push: { torrents: { $each: torrentIds } } });
  } catch (error) {
    logger.error(error);
    // eslint-disable-next-line unicorn/no-process-exit
    process.exit();
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
