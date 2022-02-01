/* eslint-disable security/detect-non-literal-fs-filename */
import express from 'express';
import morgan from 'morgan';
import amqp from 'amqplib';
import cors from 'cors';
import helmet from 'helmet';
import fs from 'fs-extra';
import torrentRouter from './routers/torrent.router';
// import videoRouter from './routers/video.router';
import notFoundHandler from './middlewares/notFoundHandler';
import errorHandler from './middlewares/errorHandler';
import connectMongo from './config/mongo';
// import connectPublisher from './rabbitmq/publisher.channel';
// import connectConsumer from './rabbitmq/consumer.torrent.channel';
import { TorrentPath } from './@types';
import { clearTorrents } from './utils/query';
import { TorrentModel } from './models/torrent.schema';

// eslint-disable-next-line import/no-mutable-exports
export let rabbitMqPublisher: amqp.Channel;

const PORT = 3000;

const app = express();

app.use(helmet());
app.use(morgan('tiny'));
app.use(cors());
app.use(express.json());

app.use('/torrent', torrentRouter);
// app.use('/video', videoRouter);

app.get('/test', async (req, res) => {
  // rabbitMqPublisher.sendToQueue('download-torrent', Buffer.from('hello'));
  // const doc =
  const doc = await TorrentModel.find({});
  res.send(doc);
});

app.listen(PORT, async () => {
  try {
    await fs.emptyDir(TorrentPath.DOWNLOAD);
    await fs.emptyDir(TorrentPath.TMP);
    await connectMongo();
    await clearTorrents();

    console.log(`Example app listening on port ${PORT}`);
  } catch (error) {
    console.error(error);
  }
});

app.use(notFoundHandler);
app.use(errorHandler);
