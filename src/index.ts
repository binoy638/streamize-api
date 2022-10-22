/* eslint-disable sonarjs/cognitive-complexity */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import express from 'express';
import morgan from 'morgan';
import cors from 'cors';
import cookieSession from 'cookie-session';
import helmet from 'helmet';
import jwt from 'jsonwebtoken';
import 'reflect-metadata';
// import fs from 'fs-extra';
import dotenv from 'dotenv';
import http from 'http';
import { Server } from 'socket.io';
import { ApolloServer } from 'apollo-server-express';
import { ApolloServerPluginLandingPageLocalDefault } from 'apollo-server-core';
import { buildSchema } from 'type-graphql';
import torrentRouter from './routers/torrent.router';
import videoRouter from './routers/video.router';
import notFoundHandler from './middlewares/notFoundHandler';
import errorHandler from './middlewares/errorHandler';
import connectMongo from './config/mongo';
import { TorrentState, UserPayload, VideoState } from './@types';
import logger from './config/logger';
// import * as rabbitMQ from './rabbitmq';
import { TorrentModel } from './models/torrent.schema';
import authRouter from './routers/auth.router';
import { UserModel } from './models/user.schema';
import mediaShareRouter from './routers/mediaShare.router';
// import { UserVideoProgressModel } from './models/userVideoProgress.schema';
// import { MediaShareModel } from './models/MediaShare';
import {
  VideoResolver,
  TorrentResolver,
  UserResolver,
  SharedPlaylistResolver,
  WatchPartyResolver,
} from './graphql/resolvers/resolver';
import socketHandler from './libs/socketHandler';

dotenv.config();

const isDevelopment = process.env.NODE_ENV === 'development';

const PORT = 3000;

(async () => {
  const apolloServer = new ApolloServer({
    schema: await buildSchema({
      resolvers: [TorrentResolver, VideoResolver, UserResolver, SharedPlaylistResolver, WatchPartyResolver],
      validate: false,
    }),
    context: ({ req }) => {
      let user: undefined | UserPayload;
      const JWTcookies = req.session?.jwt;
      if (JWTcookies) {
        const payload = jwt.verify(JWTcookies, process.env.JWT_SECRET!) as UserPayload;
        user = payload;
      }

      return { user };
    },
    formatError(error) {
      logger.error(`Apollo Server Error ${JSON.stringify(error)}`);
      return error;
    },
    debug: isDevelopment,
    plugins:
      process.env.NODE_ENV === 'development'
        ? [ApolloServerPluginLandingPageLocalDefault({ footer: false, embed: true })]
        : [],
  });
  await apolloServer.start();

  const app = express();

  const server = http.createServer(app);
  const io = new Server(server, { cors: { origin: true, credentials: true } });
  // ['http://localhost:3000', 'http://127.0.0.1:5347'];

  app.use(
    helmet({
      crossOriginEmbedderPolicy: !isDevelopment,
      contentSecurityPolicy: !isDevelopment,
    })
  );
  if (isDevelopment) {
    app.use(morgan('common'));
  }
  app.set('trust proxy', true);

  app.use(
    cookieSession({
      secret: process.env.COOKIE_SECRET!,
      maxAge: 24 * 60 * 60 * 1000 * 7,
      sameSite: 'none',
      secure: !isDevelopment,
      httpOnly: !isDevelopment,
    })
  );

  app.use(
    cors({
      origin: ['http://localhost:3000', 'https://studio.apollographql.com', process.env.ORIGIN_URL!],
      credentials: true,
    })
  );
  app.use(express.json());

  app.use('/torrent', torrentRouter);
  app.use('/video', videoRouter);
  app.use('/auth', authRouter);
  app.use('/share', mediaShareRouter);

  io.on('connection', socketHandler);

  apolloServer.applyMiddleware({
    app,
    cors: false,
  });

  server.listen(PORT, async () => {
    try {
      await connectMongo();
      //* change all incomplete torrent status to error

      await TorrentModel.updateMany(
        { $or: [{ status: TorrentState.DOWNLOADING }, { status: TorrentState.ADDED }] },
        { status: TorrentState.ERROR }
      );

      //* change the status of videos which were downloading to error (as the torrent will be removed while app crash)
      await TorrentModel.updateMany(
        {},
        { $set: { 'files.$[elem].status': VideoState.ERROR } },
        {
          arrayFilters: [{ 'elem.status': VideoState.DOWNLOADING }],
        }
      );

      //* change the status of videos which were processing to queued so they get processed from the beginning

      await TorrentModel.updateMany(
        {},
        { $set: { 'files.$[elem].status': VideoState.QUEUED } },
        {
          arrayFilters: [{ 'elem.status': VideoState.PROCESSING }],
        }
      );

      if (process.env.NODE_ENV === 'development') {
        // await fs.emptyDir(TorrentPath.DOWNLOAD);
        // await fs.emptyDir(TorrentPath.TMP);
        // await fs.emptyDir(TorrentPath.SUBTITLES);
        // rabbitMQ.publisherChannel.ackAll();
        // rabbitMQ.torrentChannel.ackAll();
        // rabbitMQ.cpuIntensiveVideoProcessingChannel.ackAll();
        // rabbitMQ.videoInspectionChannel.ackAll();
        // rabbitMQ.fileManagerChannel.ackAll();
        // rabbitMQ.nonCpuIntensiveVideoProcessingChannel.ackAll();
        // await TorrentModel.deleteMany({});
        // await UserVideoProgressModel.deleteMany({});
        // await MediaShareModel.deleteMany({});
      }
      logger.info(`🚀 Listening at http://localhost:${PORT}`);
    } catch (error) {
      logger.error(error);
    }
    try {
      if (
        !process.env.ADMIN_USER ||
        !process.env.ADMIN_PASSWORD ||
        !process.env.JWT_SECRET ||
        !process.env.COOKIE_SECRET
      )
        throw new Error('Admin credentials not set');

      const existingAdmin = await UserModel.findOne({ username: process.env.ADMIN_USER });
      if (!existingAdmin) {
        await UserModel.create({
          username: process.env.ADMIN_USER,
          password: process.env.ADMIN_PASSWORD,
          isAdmin: true,
        });
      }

      if (process.env.GUEST_USER && process.env.GUEST_PASSWORD && process.env.GUEST_STORAGE) {
        const guestUser = await UserModel.findOne({ username: 'guest' });

        if (!guestUser) {
          await UserModel.create({
            username: process.env.GUEST_USER,
            password: process.env.GUEST_PASSWORD,
            allocatedMemory: process.env.GUEST_STORAGE,
          });
        }
      }
    } catch (error) {
      logger.error(error);
      // eslint-disable-next-line unicorn/no-process-exit
      process.exit();
    }
  });
  app.on('error', error => logger.error(error));
  app.use(notFoundHandler);
  app.use(errorHandler);
})();

process.on('uncaughtException', error => {
  logger.error(error);
});

process.on('unhandledRejection', error => {
  logger.error(error);
});
