/* eslint-disable sonarjs/no-identical-functions */
import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import fs from 'fs-extra';
import { TorrentPath } from '../@types';
import logger from '../config/logger';
import Utils from '../utils';
import { UserVideoProgressModel } from '../models/userVideoProgress.schema';

export const streamVideo = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug, filename } = req.params;
  const path = `${TorrentPath.DOWNLOAD}/${videoSlug}/${filename}`;

  const exists = await fs.pathExists(path);

  if (!exists) {
    next(boom.notFound('video not found'));
    return;
  }

  try {
    res.sendFile(path, error => {
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
    return;
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

export const getPreview = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug, filename } = req.params;
  const path = `${TorrentPath.DOWNLOAD}/${videoSlug}/thumbnails/${filename}`;
  const exists = await fs.pathExists(path);

  if (!exists) {
    next(boom.notFound('file not found'));
    return;
  }

  try {
    res.header('Access-Control-Allow-Origin', '*');
    res.header('Cross-Origin-Resource-Policy', 'cross-origin');

    res.sendFile(path, err => {
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
    return;
  }

  const video = await Utils.getVideoFile(videoSlug, false);
  if (!video) {
    next(boom.notFound('video not found'));
    return;
  }
  res.status(200).send(video);
};

export const saveUserVideoProgress = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug } = req.params;
  const { progress } = req.body;
  const { currentUser } = req;

  try {
    await UserVideoProgressModel.findOneAndUpdate(
      { user: currentUser.id, video: videoSlug },
      { progress },
      { upsert: true, new: true }
    );
    res.sendStatus(204);
  } catch (error) {
    logger.error(error);

    next(boom.internal('Internal server error'));
  }
};

export const getUserVideoProgress = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { videoSlug } = req.params;
  const { currentUser } = req;

  try {
    const doc = await UserVideoProgressModel.findOne({ user: currentUser.id, video: videoSlug });
    if (doc && doc.progress > 0) {
      res.status(200).send({ progress: doc.progress });
      return;
    }
    res.sendStatus(404);
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
