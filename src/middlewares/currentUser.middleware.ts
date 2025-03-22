/* eslint-disable @typescript-eslint/no-namespace */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import boom from '@hapi/boom';
import { NextFunction, Request, Response } from 'express';
import jwt from 'jsonwebtoken';
import { UserPayload } from '../@types';
import logger from '../config/logger';

declare global {
  namespace Express {
    interface Request {
      currentUser: UserPayload;
    }
  }
}

export const getCurrentUser = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  if (!req.session?.jwt) {
    logger.error("jwt not found")
    next(boom.unauthorized('Not authorized'));
    return;
  }

  try {
    const payload = jwt.verify(req.session.jwt, process.env.JWT_SECRET!) as UserPayload;
    req.currentUser = payload;
    next();
  } catch (error) {
    logger.error(error);
    logger.error("error while verifying jwt")
    next(boom.unauthorized('Not authorized'));
  }
};
