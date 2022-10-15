import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import logger from '../config/logger';
import { WatchPartyModel } from '../models/watchParty';

export const create = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { maxViewers, partyPlayerControl, name } = req.body;

  const { currentUser } = req;
  try {
    const doc = await WatchPartyModel.create({
      host: currentUser.id,
      name,
      partyPlayerControl,
      maxViewers,
    });
    res.status(201).send(doc);
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const getWatchParty = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { slug } = req.params;

  try {
    const watchParty = await WatchPartyModel.findOne({
      slug,
    }).lean();
    if (!watchParty) {
      next(boom.notFound('Watch party not found'));
      return;
    }
    res.status(200).send({ data: watchParty });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
