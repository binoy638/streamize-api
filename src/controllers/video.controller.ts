import { Request, Response } from 'express';
import boom from '@hapi/boom';
import 'express-async-errors';
import fs from 'fs';
import { getTorrentBySlug, getVideoFile } from '../utils/query';

export const getVideo = async (req: Request, res: Response): Promise<void> => {
  const { torrentSlug, videoSlug } = req.params;
  if (!torrentSlug || !videoSlug) {
    throw boom.badRequest('torrentSlug and videoSlug is required');
  }

  const video = await getVideoFile(torrentSlug, videoSlug);
  if (!video) {
    throw boom.notFound('video not found');
  }

  // Ensure there is a range given for the video
  const { range } = req.headers;
  if (!range) {
    throw boom.badRequest('range is required');
  }

  const videoPath = video.path;
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

export const getTorrentVideos = async (req: Request, res: Response): Promise<void> => {
  const { slug } = req.params;
  if (!slug) {
    throw boom.badRequest('slug is required');
  }

  const torrent = await getTorrentBySlug(slug);
  if (!torrent) {
    throw boom.notFound('torrent not found');
  }
  res.send({ files: torrent.files });
};
