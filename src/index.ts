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

// eslint-disable-next-line import/no-mutable-exports

const PORT = 3000;

const app = express();

app.use(helmet());
app.use(morgan('tiny'));
app.use(cors());
app.use(express.json());

app.use('/torrent', torrentRouter);
app.use('/video', videoRouter);

// app.get('/test', async (req, res) => {
//   // rabbitMqPublisher.sendToQueue('download-torrent', Buffer.from('hello'));
//   const doc = await TorrentModel.find({});
//   // const doc = await TorrentModel.updateOne({ _id: '61fab50baf98fd0e99876b45' }, { $set: { 'files.$.status':  } });
//   // console.log(doc);
//   // const doc = await TorrentModel.find({});
//   // logger.info('hello file:%o', { hello: 'hi' });
//   res.send(doc);
// });

app.listen(PORT, async () => {
  try {
    await fs.emptyDir(TorrentPath.DOWNLOAD);
    await fs.emptyDir(TorrentPath.TMP);
    await connectMongo();
    await publisherChannel.cancelAll();
    await torrentChannel.cancelAll();
    await videoChannel.cancelAll();
    await fileManagerChannel.cancelAll();
    await clearTorrents();

    console.log(`Example app listening on port ${PORT}`);
  } catch (error) {
    console.error(error);
  }
});

app.on('error', error => logger.error(error));

app.use(notFoundHandler);
app.use(errorHandler);
