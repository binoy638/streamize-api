import ffmpeg from 'fluent-ffmpeg';
import redisClient from '../config/redis';
import client from '../config/webtorrent';

export const convertMKVtoMp4 = (
  filePath: string,
  outputPath: string,
  torrentID: string,
  filename: string
): Promise<void> => {
  return new Promise<void>((resolve, reject) => {
    ffmpeg(filePath)
      .audioCodec('copy')
      .videoCodec('copy')
      .output(outputPath)
      .on('start', () => {
        console.log('started..');
      })
      .on('end', async () => {
        const torrent = client.get(torrentID);
        if (torrent) {
          torrent.destroy({ destroyStore: true });
        }
        const data = { progress: 100, done: true };
        redisClient.set(torrentID, JSON.stringify(data));
        const key = 'VideoList';
        let videoList = (await redisClient.get(key)) || [];

        if (typeof videoList === 'string') {
          videoList = JSON.parse(videoList);
        }
        const filenameWithExt = filename.concat('.mp4');
        const video = { id: torrentID, name: filenameWithExt, path: outputPath };
        if (typeof videoList === 'object') {
          videoList.push(video as never);
        }
        console.log(videoList);
        redisClient.set(key, JSON.stringify(videoList));

        console.log('ended..');
        resolve();
      })
      .on('error', err => () => {
        const torrent = client.get(torrentID);
        if (torrent) {
          torrent.destroy({ destroyStore: true });
        }
        redisClient.del(torrentID);
        console.log({ err });
        reject(err);
      })
      .on('progress', progress => {
        console.log('insode progress');
        const data = { progress: progress.percent, done: false };
        redisClient.set(torrentID, JSON.stringify(data));
        console.log(`Processing: ${progress.percent}% done`);
      })
      .run();
  });
};
