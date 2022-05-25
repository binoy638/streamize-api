import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import fs from 'fs-extra';
import { getVideoFile } from '../utils/query';
import { TorrentPath } from '../@types';
import logger from '../config/logger';

export const streamVideo = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug, filename } = req.params;

  const path = `${TorrentPath.DOWNLOAD}/${videoSlug}/${filename}`;

  const options = {
    headers: {
      'Access-Control-Allow-Origin': '*',
    },
  };
  try {
    res.sendFile(path, options, error => {
      if (error) {
        logger.error(error);
      }
    });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const getSubtitle = async (req: Request, res: Response): Promise<void> => {
  const { videoSlug, filename } = req.query;
  if (!filename || !videoSlug) {
    throw boom.badRequest('filename is required');
  }
  const exists = await fs.pathExists(`${TorrentPath.SUBTITLES}/${videoSlug}/${filename}`);

  if (!exists) {
    throw boom.notFound('file not found');
  }
  const options = {
    root: TorrentPath.SUBTITLES,
  };
  res.sendFile(filename as string, options, err => {
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

// export const downloadVideo = async (req: Request, res: Response): Promise<void> => {
//   const { videoSlug } = req.params;
//   if (!videoSlug) {
//     throw boom.badRequest('filename is required');
//   }

//   const video = await getVideoFile(videoSlug, true);
//   if (!video) {
//     throw boom.notFound('video not found');
//   }
//   const { name, path } = video;

//   const options = {
//     headers: {
//       'Content-Disposition': `attachment; filename=${name}`,
//     },
//   };
//   res.download(path, name, options, error => {
//     if (error) {
//       logger.error(error);
//     }
//   });
// };
