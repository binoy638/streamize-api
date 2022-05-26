import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import fs from 'fs-extra';
import { TorrentPath } from '../@types';
import logger from '../config/logger';
import Utils from '../utils';

export const streamVideo = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug, filename } = req.params;

  const path = `${TorrentPath.DOWNLOAD}/${videoSlug}/${filename}`;

  const exists = await fs.pathExists(path);

  if (!exists) {
    next(boom.notFound('video not found'));
    return;
  }

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

export const getSubtitle = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug, filename } = req.params;

  const exists = await fs.pathExists(`${TorrentPath.SUBTITLES}/${videoSlug}/${filename}`);

  if (!exists) {
    next(boom.notFound('file not found'));
  }
  const options = {
    headers: {
      'Access-Control-Allow-Origin': '*',
    },
    root: `${TorrentPath.SUBTITLES}/${videoSlug}`,
  };
  try {
    res.sendFile(filename as string, options, err => {
      if (err) {
        console.log(err);
      } else {
        console.log('Sent:', filename);
      }
    });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const getVideoInfo = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug } = req.params;
  if (!videoSlug) {
    next(boom.badRequest('filename is required'));
  }

  const video = await Utils.getVideoFile(videoSlug, false);
  if (!video) {
    next(boom.notFound('video not found'));
  }
  res.status(200).send(video);
};
