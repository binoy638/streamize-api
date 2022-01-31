import { Request, Response } from 'express';
import fs from 'fs';
import 'express-async-errors';

export const getVideo = async (req: Request, res: Response): Promise<void> => {
  const { id } = req.params;
  // Ensure there is a range given for the video
  const { range } = req.headers;
  if (!range) {
    res.status(400).send('Requires Range header');
  } else {
    const videoList = await redisClient.get('VideoList');
    if (!videoList) {
      res.status(400).send('video not available');
    } else {
      const videoListParsed = JSON.parse(videoList);

      const videoData = videoListParsed.find((vid: any) => vid.id === id);
      //! Hanle exception and do typing
      const videoPath = videoData.path;
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
    }
  }
};

export const getAllAvailableVideos = async (req: Request, res: Response): Promise<void> => {
  const videoList = await redisClient.get('VideoList');
  if (videoList) {
    res.send({ data: JSON.parse(videoList) });
  } else {
    res.status(404).send({ data: [] });
  }
};
