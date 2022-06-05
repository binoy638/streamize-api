/* eslint-disable @typescript-eslint/no-namespace */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import boom from '@hapi/boom';
import { NextFunction, Request, Response } from 'express';
import logger from '../config/logger';
import { MediaShareModel } from '../models/MediaShare';

export const verifyMediaShare = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { slug } = req.params;

  if (!slug) {
    next(boom.badRequest('id is required'));
    return;
  }

  try {
    const sharedMedia = await MediaShareModel.findOne({ slug, expiresIn: { $gt: new Date() } });
    if (!sharedMedia) {
      next(boom.notFound('Media not found'));
      return;
    }
    //! need to check if the torrent and video exists
    next();
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
