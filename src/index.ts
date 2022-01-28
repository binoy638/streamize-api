import express from 'express';
import morgan from 'morgan';
import cors from 'cors';
import helmet from 'helmet';
import path from 'path';
import fs from 'fs';
import client from './config/webtorrent';
import { getDataFromTorrent } from './utils/torrentHelper';
import redisClient from './config/redis';
import { convertMKVtoMp4 } from './utils/videoConverter';

const PORT = 3000;

const app = express();

app.use(helmet());
app.use(morgan('tiny'));
app.use(cors({ origin: '*' }));
app.use(express.json());

const allowedExt = ['mp4', 'mkv', 'avi'];

app.post('/addmagnet', async (req, res) => {
  try {
    const { magnet } = req.body;

    client.add(magnet, { destroyStoreOnDestroy: true, path: 'src/tmp' }, torrent => {
      const files = torrent.files.map(file => {
        return { name: file.name, path: file.path };
      });

      const isMediaFile = files.some(file => {
        const ext = file.name.split('.').pop();
        if (ext) {
          return allowedExt.includes(ext);
        }
        return false;
      });

      if (!isMediaFile) {
        torrent.destroy({ destroyStore: true });
        res.status(404).send({ error: 'Invalid file type' });
      } else {
        const convertable = files.some(file => file.name.split('.').pop() === 'mkv');
        if (convertable) {
          torrent.on('done', () => {
            const mkvfile = files.find(file => file.name.split('.').pop() === 'mkv');
            if (mkvfile) {
              const fileNameWithoutExt = path.parse(mkvfile.name).name;
              console.log({ fileNameWithoutExt });
              convertMKVtoMp4(
                mkvfile.path,
                `/home/app/src/videos/${fileNameWithoutExt}.mp4`,
                torrent.infoHash,
                fileNameWithoutExt
              ).catch(err => console.log(err));
            }
          });
        }
        res.send({
          message: 'Torrent is downloading',
          InfoHash: torrent.infoHash,
          files,
          convertable,
        });
      }
    });
  } catch (error) {
    console.log({ error });
  }
});

app.get('/torrent/info/:id', async (req, res) => {
  const { id } = req.params;
  const torrent = client.get(id);
  if (torrent) {
    res.send(getDataFromTorrent(torrent));
  } else {
    res.status(404).send({ message: 'torrent not found' });
  }
});

app.get('/torrent/convert/:id', async (req, res) => {
  const { id } = req.params;
  const data = await redisClient.get(id);
  if (data) {
    res.send(JSON.parse(data));
  } else {
    res.status(404).send({ message: 'torrent not found' });
  }
});

app.listen(PORT, async () => {
  await redisClient.connect();
  console.log(`Example app listening on ports ${PORT}`);
});

app.get('/torrent/delete/:id', (req, res) => {
  const { id } = req.params;

  const torrent = client.get(id);
  if (torrent) {
    torrent.destroy({}, () => {
      res.send({ message: 'torrent deleted' });
    });
  } else {
    res.status(404).send({ message: 'torrent not found' });
  }
});

app.get('/torrent/all', (req, res) => {
  const torrents = client.torrents.map(torrent => torrent.infoHash);

  res.send({ torrents });
});

// convertMKVtoMp4('/home/app/src/video1.mkv', '/home/app/src/exm.mp4');

app.get('/video/:id', async (req, res) => {
  const { id } = req.params;
  // Ensure there is a range given for the video
  const range = req.headers.range;
  if (!range) {
    res.status(400).send('Requires Range header');
  } else {
    const videoList = await redisClient.get('VideoList');
    if (!videoList) {
      res.status(400).send('video not available');
    } else {
      const videoListParsed = JSON.parse(videoList);

      const videoData = videoListParsed.find(v => v.id === id);
      //!Hanle exception and do typing
      const videoPath = videoData.path;
      const videoSize = fs.statSync(videoPath).size;

      // Parse Range
      // Example: "bytes=32324-"
      const CHUNK_SIZE = 10 ** 6; // 1MB
      const start = Number(range.replace(/\D/g, ''));
      const end = Math.min(start + CHUNK_SIZE, videoSize - 1);

      // Create headers
      const contentLength = end - start + 1;
      const headers = {
        'Content-Range': `bytes ${start}-${end}/${videoSize}`,
        'Accept-Ranges': 'bytes',
        'Content-Length': contentLength,
        'Content-Type': 'video/mp4',
        'Access-Control-Allow-Origin': '*',
        'Cross-Origin-Resource-Policy': 'cross-origin',
      };

      // HTTP Status 206 for Partial Content
      res.writeHead(206, headers);

      // create video read stream for this particular chunk
      const videoStream = fs.createReadStream(videoPath, { start, end });

      // Stream the video chunk to the client
      videoStream.pipe(res);
    }
  }
});

app.get('/videos', async (req, res) => {
  const videoList = await redisClient.get('VideoList');
  if (videoList) {
    res.send({ data: JSON.parse(videoList) });
  } else {
    res.status(404).send({ data: [] });
  }
});
