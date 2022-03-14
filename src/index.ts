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

app.get('/test', async (req, res) => {
  const num = (req.query.num as unknown as number) || 1;
  // eslint-disable-next-line security/detect-non-literal-fs-filename
  fs.readdir(TorrentPath.DOWNLOAD, (err, files) => {
    // eslint-disable-next-line security/detect-object-injection
    const file = files[num];
    const name = file.split('/').pop() || 'abc';
    const options = {
      headers: {
        'Content-Disposition': `attachment; filename=${name}`,
      },
    };
    // eslint-disable-next-line security/detect-object-injection
    res.download(files[num], name, options, error => {
      if (error) {
        logger.error(error);
      }
    });
  });
});
app.listen(PORT, async () => {
  try {
    await connectMongo();
    await redisClient.connect();
    if (process.env.NODE_ENV === 'development') {
      await fs.emptyDir(TorrentPath.DOWNLOAD);
      await fs.emptyDir(TorrentPath.TMP);
      await fs.emptyDir(TorrentPath.SUBTITLES);
      publisherChannel.ackAll();
      torrentChannel.ackAll();
      videoChannel.ackAll();
      fileManagerChannel.ackAll();
      await clearTorrents();
    }

    logger.info(`Example app listening on port ${PORT}`);
  } catch (error) {
    logger.error(error);
  }
});

app.on('error', error => logger.error(error));

app.use(notFoundHandler);
app.use(errorHandler);
