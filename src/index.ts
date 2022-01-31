import express from 'express';
import morgan from 'morgan';
import amqp from 'amqplib';
import cors from 'cors';
import helmet from 'helmet';
import torrentRouter from './routers/torrent.router';
import videoRouter from './routers/video.router';
import notFoundHandler from './middlewares/notFoundHandler';
import errorHandler from './middlewares/errorHandler';
import connectMongo from './config/mongo';
import connectPublisher from './rabbitmq/publish';
import connectConsumer from './rabbitmq/consumer';
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
  try {
  } catch (error) {
    console.log(error);
  }
  res.send({ msg: 'ok' });
});

app.listen(PORT, async () => {
  try {
    await connectMongo();
    rabbitMqPublisher = await connectPublisher();
    await connectConsumer(rabbitMqPublisher);
    console.log(`Example app listening on ports ${PORT}`);
  } catch (error) {
    console.error(error);
  }
});

app.use(notFoundHandler);
app.use(errorHandler);
   // "pre-commit": "npm run typecheck && lint-staged"