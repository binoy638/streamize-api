/* eslint-disable sonarjs/cognitive-complexity */
/* eslint-disable prefer-promise-reject-errors */
import ffmpeg from 'fluent-ffmpeg';
import { IVideo } from '../@types';
import logger from '../config/logger';
import { updateFileConvertProgress, updateTorrentFileStatus, updateTorrentInfo } from './query';

// TODO: multiple audio support

const supportedAudioCodecs = new Set(['aac', 'flac', 'mp3', 'ogg', 'opus', 'mpeg', 'mpeg-1', 'mpeg-2']);

const supportedVideoCodecs = new Set(['avc', 'h264', 'theora', 'vp8', 'vp9']);

// chrome supported audio codecs:  aac,flac,mp3,ogg,opus,mpeg,mpeg-1,mpeg-2
// chrome supported video codecs: avc,h264,theora,vp8,vp9,
export const getSupportedCodecs = (filePath: string): Promise<[string, string]> =>
  new Promise<[string, string]>((resolve, reject) => {
    let audioCodec = 'libmp3lame';
    let videoCodec = 'libx264';
    ffmpeg(filePath).ffprobe((error, data) => {
      if (!error) {
        const audioStreams = data.streams.filter(stream => stream.codec_type === 'audio');
        const videoStreams = data.streams.filter(stream => stream.codec_type === 'video');
        if (audioStreams.length > 0) {
          const audioStream = audioStreams[0];
          if (audioStream?.codec_name && supportedAudioCodecs.has(audioStream.codec_name)) {
            logger.info(`Audio codec ${audioStream.codec_name} is supported`);
            audioCodec = 'copy';
          } else {
            logger.info(`Audio codec ${audioStream?.codec_name} is not supported`);
          }
        }
        if (videoStreams.length > 0) {
          const videoStream = videoStreams[0];
          if (videoStream?.codec_name && supportedVideoCodecs.has(videoStream.codec_name)) {
            logger.info(`Video codec ${videoStream.codec_name} is supported`);
            videoCodec = 'copy';
          } else {
            logger.info(`Video codec ${videoStream?.codec_name} is not supported`);
          }
        }
        resolve([audioCodec, videoCodec]);
      } else {
        reject(error);
      }
    });
  });

export const convertMKVtoMp4 = async (
  filePath: string,
  outputPath: string,
  slug: string,
  torrentId: string
): Promise<string> => {
  const [audioCodec, videoCodec] = await getSupportedCodecs(filePath);
  return new Promise<string>((resolve, reject) => {
    ffmpeg(filePath)
      .format('mp4')
      .audioCodec(audioCodec)
      .videoCodec(videoCodec)
      .output(outputPath)
      .on('start', async () => {
        await updateTorrentInfo(torrentId, { status: 'converting' });
        await updateTorrentFileStatus(torrentId, slug, 'converting');
        logger.info('converting video started');
      })
      .on('end', async () => {
        logger.info('conversion done');
        await updateTorrentInfo(torrentId, { status: 'done' });
        await updateTorrentFileStatus(torrentId, slug, 'done');
        await updateFileConvertProgress(torrentId, slug, 100, 'done');
        resolve(slug);
      })
      .on('error', async err => {
        await updateTorrentInfo(torrentId, { status: 'error' });
        await updateTorrentFileStatus(torrentId, slug, 'error');
        await updateFileConvertProgress(torrentId, slug, 0, 'error');
        logger.error(err.message);
        reject('something went wrong');
      })
      .on('progress', async progress => {
        logger.debug(`converting video progress: ${progress.percent}`);
        await updateFileConvertProgress(torrentId, slug, progress.percent, 'processing');
      })
      .run();
  });
};

export const isFileConvertable = async (file: IVideo): Promise<boolean> => {
  if (file.ext !== 'mp4') return true;
  const [audioCodec, videoCodec] = await getSupportedCodecs(file.path);
  if (audioCodec === 'copy' && videoCodec === 'copy') return false;
  return true;
};
