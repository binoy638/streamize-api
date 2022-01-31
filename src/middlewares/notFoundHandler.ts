import type { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';

export default (req: Request, res: Response, next: NextFunction): void => {
  next(boom.notFound('The requested resource does not exist.'));
};
