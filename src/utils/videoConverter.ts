import ffmpeg from 'fluent-ffmpeg';
import logger from '../config/logger';
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
        logger.info('converting video started');
      })
      .on('end', async () => {
        logger.info('cconversion done');
        await updateTorrentInfo(torrentId, { status: 'done' });
        await updateFileConvertProgress(torrentId, slug, 100, 'done');
        resolve(slug);
      })
      .on('error', err => async () => {
        await updateTorrentInfo(torrentId, { status: 'error' });
        await updateFileConvertProgress(torrentId, slug, 0, 'error');
        logger.error(err);
        reject(err);
      })
      .on('progress', async progress => {
        logger.debug(`converting video progress: ${progress.percent}`);
        await updateFileConvertProgress(torrentId, slug, progress.percent, 'processing');
      })
      .run();
  });
};
