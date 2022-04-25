import { Request, Response } from 'express';
import boom from '@hapi/boom';
import 'express-async-errors';
import fs from 'fs-extra';
import { getVideoFile } from '../utils/query';
import redisClient from '../config/redis';
import { TorrentPath } from '../@types';
import logger from '../config/logger';

export const getVideo = async (req: Request, res: Response): Promise<void> => {
  const { videoSlug } = req.params;
  if (!videoSlug) {
    throw boom.badRequest(' videoSlug is required');
  }

  let videoPath = await redisClient.get(`VIDEO_PATH:${videoSlug}`);
  if (!videoPath) {
    const video = await getVideoFile(videoSlug, true);
    if (!video) {
      throw boom.notFound('video not found');
    }
    videoPath = video.path;
    await redisClient.set(`VIDEO_PATH:${videoSlug}`, videoPath);
  }

  // Ensure there is a range given for the video
  const { range } = req.headers;
  if (!range) {
    throw boom.badRequest('range is required');
  }

  // eslint-disable-next-line security/detect-non-literal-fs-filename
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
  // eslint-disable-next-line security/detect-non-literal-fs-filename
  const videoStream = fs.createReadStream(videoPath, { start, end });

  // Stream the video chunk to the client
  videoStream.pipe(res);
};

export const getSubtitle = async (req: Request, res: Response): Promise<void> => {
  const { filename } = req.params;
  if (!filename) {
    throw boom.badRequest('filename is required');
  }
  const exists = await fs.pathExists(`${TorrentPath.SUBTITLES}/${filename}`);

  if (!exists) {
    throw boom.notFound('file not found');
  }
  const options = {
    root: TorrentPath.SUBTITLES,
  };
  res.sendFile(filename, options, err => {
    if (err) {
      console.log(err);
    } else {
      console.log('Sent:', filename);
    }
  });
};

export const getVideoInfo = async (req: Request, res: Response): Promise<void> => {
  const { videoSlug } = req.params;
  if (!videoSlug) {
    throw boom.badRequest('filename is required');
  }

  const video = await getVideoFile(videoSlug, false);
  if (!video) {
    throw boom.notFound('video not found');
  }
  res.status(200).send(video);
};

export const downloadVideo = async (req: Request, res: Response): Promise<void> => {
  const { videoSlug } = req.params;
  if (!videoSlug) {
    throw boom.badRequest('filename is required');
  }

  const video = await getVideoFile(videoSlug, true);
  if (!video) {
    throw boom.notFound('video not found');
  }
  const { name, path } = video;

  const options = {
    headers: {
      'Content-Disposition': `attachment; filename=${name}`,
    },
  };
  res.download(path, name, options, error => {
    if (error) {
      logger.error(error);
    }
  });
};
