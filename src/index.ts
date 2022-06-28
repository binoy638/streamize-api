/* eslint-disable @typescript-eslint/no-non-null-assertion */
import express from 'express';
import morgan from 'morgan';
import cors from 'cors';
import cookieSession from 'cookie-session';
import helmet from 'helmet';
import jwt from 'jsonwebtoken';
import 'reflect-metadata';
import fs from 'fs-extra';
import dotenv from 'dotenv';
import http from 'http';
import { Server, Socket } from 'socket.io';
import { ApolloServer } from 'apollo-server-express';
import { buildSchema } from 'type-graphql';
import torrentRouter from './routers/torrent.router';
import videoRouter from './routers/video.router';
import notFoundHandler from './middlewares/notFoundHandler';
import errorHandler from './middlewares/errorHandler';
import connectMongo from './config/mongo';
import { SyncStreamsEvents, TorrentPath, TorrentState, UserPayload, VideoState } from './@types';
import logger from './config/logger';
import * as rabbitMQ from './rabbitmq';
import { TorrentModel } from './models/torrent.schema';
import authRouter from './routers/auth.router';
import { UserModel } from './models/user.schema';
import mediaShareRouter from './routers/mediaShare.router';
import { UserVideoProgressModel } from './models/userVideoProgress.schema';
import { MediaShareModel } from './models/MediaShare';
import { SyncStreams, Stream } from './libs/sync-streams';
import { VideoResolver, TorrentResolver, UserResolver, SharedPlaylistResolver } from './graphql/resolvers/resolver';

dotenv.config();

const isDevelopment = process.env.NODE_ENV === 'development';

const PORT = 3000;

(async () => {
  const apolloServer = new ApolloServer({
    schema: await buildSchema({
      resolvers: [TorrentResolver, VideoResolver, UserResolver, SharedPlaylistResolver],
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
    debug: isDevelopment,
  });
  await apolloServer.start();

  const app = express();

  const server = http.createServer(app);
  const io = new Server(server, { cors: { origin: true, credentials: true } });
  // ['http://localhost:3000', 'http://127.0.0.1:5347'];
  const syncStreams = new SyncStreams();

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

  const onConnection = (socket: Socket): void => {
    console.log('a user connected :', socket.id);
    socket.on(SyncStreamsEvents.CREATED, (data: Stream) => {
      socket.join(data.id);
      syncStreams.addStream(data);
    });
    socket.on(SyncStreamsEvents.NEW_MEMBER_JOINED, (data: { streamId: string; member: string }) => {
      socket.join(data.streamId);
      syncStreams.addMember(data.streamId, data.member);
    });
    socket.on(SyncStreamsEvents.PLAY, (data: { streamId: string }) => {
      console.log(data);
      socket.to(data.streamId).emit(SyncStreamsEvents.PLAY, data);
    });
    socket.on(SyncStreamsEvents.PAUSE, (data: { streamId: string }) => {
      socket.to(data.streamId).emit(SyncStreamsEvents.PAUSE);
    });
    socket.on(SyncStreamsEvents.SEEKED, (data: { streamId: string; seekTime: number }) => {
      socket.to(data.streamId).emit(SyncStreamsEvents.SEEKED, data);
    });
    socket.on('disconnect', () => {
      console.log('a user disconnected :', socket.id);
    });
  };

  io.on('connection', onConnection);
  apolloServer.applyMiddleware({
    app,
    cors: false,
  });

  server.listen(PORT, async () => {
    try {
      await connectMongo();
      //* change all incomplete torrent status to error
      await TorrentModel.updateMany({ status: TorrentState.DOWNLOADING }, { status: TorrentState.ERROR });
      await TorrentModel.updateMany(
        { 'files.$.status': VideoState.DOWNLOADING },
        { $set: { 'files.$.status': VideoState.ERROR } }
      );

      //* change all processing torrent status to queued
      await TorrentModel.updateMany(
        { 'files.$.status': VideoState.PROCESSING },
        { $set: { 'files.$.status': VideoState.QUEUED } }
      );

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
        await UserVideoProgressModel.deleteMany({});
        await MediaShareModel.deleteMany({});
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
