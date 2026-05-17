import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import logger from '../config/logger';
import { WatchPartyModel } from '../models/watchParty';

export const create = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { torrents, partyPlayerControl } = req.body;

  const { currentUser } = req;
  try {
    const doc = await WatchPartyModel.create({
      host: currentUser.id,
      torrents,
      partyPlayerControl,
    });
    res.status(201).send(doc);
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
