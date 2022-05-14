/* eslint-disable security/detect-non-literal-fs-filename */
/* eslint-disable unicorn/new-for-builtins */
/* eslint-disable prefer-promise-reject-errors */
import { ConsumeMessage } from 'amqplib';
import ffmpeg from 'fluent-ffmpeg';
import path from 'path';
import fs from 'fs-extra';
import { ISubtitle, TorrentPath } from '../@types';
import { IConvertVideoMessageContent } from '../@types/message';
import { addSubtitleFile } from './query';
import logger from '../config/logger';

export const allowedExt = new Set(['mp4', 'mkv', 'avi']);
export const convertableExt = new Set(['mkv', 'avi']);

export function getMessageContent<T>(message: ConsumeMessage): T {
  if (typeof message.content === 'string') {
    return message.content as T;
  }
  return JSON.parse(message.content.toString()) as T;
}

export const stripFileExtension = (fileName: string): string => {
  return path.parse(fileName).name;
};

export const getFileOutputPath = (fileName: string, path: string): string => {
  return `${path}/${stripFileExtension(fileName)}.mp4`;
};

export const getSubtitleOutputPath = (fileName: string, path: string): string => {
  return `${path}/${fileName}`;
};

interface Sub {
  title: string;
  language: string;
}

export const getSubtitle = (
  inputVideoFile: string,
  subtitleInfo: ISubtitle,
  index: number,
  torrentID: string,
  videoSlug: string
): Promise<void> =>
  new Promise<void>((resolve, reject) => {
    ffmpeg(inputVideoFile)
      .noAudio()
      .noVideo()
      .outputOptions('-map', `0:s:${index}`)
      .output(subtitleInfo.path)
      .on('end', () => {
        logger.info(`Subtitle extracted successfully ${subtitleInfo}`);
        addSubtitleFile(torrentID, videoSlug, subtitleInfo)
          .then(() => resolve())
          .catch(error => {
            logger.error(error);
            reject(error);
          });
      })
      .on('error', error => {
        logger.error(`Subtitle extraction failed ${subtitleInfo}`);
        logger.error(error.message);
        reject();
      })
      .run();
  });

export const getSubtitlesList = (videoFilePath: string): Promise<Sub[]> =>
  new Promise<Sub[]>((resolve, reject) => {
    ffmpeg(videoFilePath).ffprobe((err, data) => {
      if (err) {
        logger.error(err);
        reject(err);
      } else {
        const subtitlesCodecs = data.streams
          .filter(stream => stream.codec_type === 'subtitle')
          .map(subtitle => {
            return {
              title: subtitle?.tags?.title || 'unknown',
              language: subtitle?.tags?.language || subtitle?.tags?.LANGUAGE || 'unknown',
            };
          });
        resolve(subtitlesCodecs);
      }
    });
  });

export const extractSubtitles = async (videoFile: IConvertVideoMessageContent): Promise<void> => {
  const subtitleslist = await getSubtitlesList(videoFile.path).catch(error => {
    logger.error(error);
  });
  if (subtitleslist && subtitleslist.length > 0) {
    const promises = subtitleslist.map((subtitle, index) => {
      const fileName = `${videoFile.slug}-${subtitle.language}-${index}.vtt`;
      const subtitleInfo: ISubtitle = {
        fileName,
        title: subtitle.title,
        language: subtitle.language,
        path: getSubtitleOutputPath(fileName, TorrentPath.SUBTITLES),
      };
      return getSubtitle(videoFile.path, subtitleInfo, index, videoFile.torrentID, videoFile.slug);
    });

    await Promise.allSettled(promises).catch(error => {
      logger.error(error);
    });
  }
};

export const isEmpty = async (dirPath: string): Promise<boolean> => {
  // eslint-disable-next-line security/detect-non-literal-fs-filename
  const files = await fs.readdir(dirPath);
  if (files.length === 0) return true;
  // eslint-disable-next-line no-restricted-syntax
  for (const file of files) {
    const filePath = `${dirPath}/${file}`;
    // eslint-disable-next-line no-await-in-loop
    const stats = await fs.stat(filePath);
    if (stats.isDirectory()) {
      return isEmpty(`${dirPath}/${file}`);
    }
    return false;
  }
  return false;
};
