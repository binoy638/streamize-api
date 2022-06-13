import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import logger from '../config/logger';
import { MediaShareModel } from '../models/MediaShare';
import { ITorrent } from '../@types';
import { UserDoc } from '../models/user.schema';

export const create = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { body } = req;

  const { currentUser } = req;
  try {
    const doc = await MediaShareModel.create({
      ...body,
      user: currentUser.id,
      expiresIn: new Date(Date.now() + 1000 * 60 * 60 * 24 * 7),
    });
    res.status(201).send(doc);
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const getPlaylist = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { slug } = req.params;
  try {
    const sharedMedia = await MediaShareModel.findOne({ slug })
      .populate<{ torrent: ITorrent; user: UserDoc }>('torrent user')
      .lean();
    if (!sharedMedia) {
      next(boom.notFound('Media not found'));
      return;
    }
    if (sharedMedia.isTorrent) {
      res.send({ torrent: sharedMedia.torrent, user: sharedMedia.user.username });
      return;
    }
    const filteredTorrentFiles = sharedMedia.torrent.files.filter(file => file.slug === sharedMedia.mediaId);
    res.send({ torrent: filteredTorrentFiles, user: sharedMedia.user.username });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
