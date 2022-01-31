import ffmpeg from 'fluent-ffmpeg';
import { updateFileConvertProgress, updateTorrentInfo } from './query';

export const convertMKVtoMp4 = (
  filePath: string,
  outputPath: string,
  slug: string,
  torrentId: string
): Promise<string> => {
  return new Promise<string>((resolve, reject) => {
    ffmpeg(filePath)
      .format('mp4')
      .audioCodec('libmp3lame')
      .videoCodec('copy')
      .output(outputPath)
      .on('start', () => {
        updateTorrentInfo(torrentId, { status: 'converting' });
        console.log('started..');
      })
      .on('end', async () => {
        console.log('conversion done');
        updateTorrentInfo(torrentId, { status: 'done' });
        reject(slug);
      })
      .on('error', err => () => {
        updateTorrentInfo(torrentId, { status: 'error' });
        console.log(err);
        reject(err);
      })
      .on('progress', async progress => {
        console.log(`${progress.percent}%`);
        await updateFileConvertProgress(torrentId, slug, progress.percent, 'processing');
        resolve(slug);
      })
      .run();
  });
};
