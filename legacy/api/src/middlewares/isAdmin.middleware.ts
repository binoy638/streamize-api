/* eslint-disable @typescript-eslint/no-namespace */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import boom from '@hapi/boom';
import { NextFunction, Request, Response } from 'express';

export const isAdmin = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { currentUser } = req;

  if (!currentUser || !currentUser.isAdmin) {
    next(boom.unauthorized('Not authorized'));
    return;
  }

  next();
};
