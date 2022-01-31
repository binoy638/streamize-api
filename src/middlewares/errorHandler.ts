/* eslint-disable @typescript-eslint/no-unused-vars */
import type { Request, Response, NextFunction } from 'express';
import boom from '@hapi/boom';

// eslint-disable-next-line no-unused-vars
export default (err: Error, req: Request, res: Response, next: NextFunction): void => {
  const {
    output: { payload: error, statusCode },
  } = boom.boomify(err);
  res.status(statusCode).json({ error });
  if (statusCode >= 500) {
    console.error(err);
  }
};
