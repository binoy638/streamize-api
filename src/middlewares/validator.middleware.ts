import boom from '@hapi/boom';
import { NextFunction, Request, Response } from 'express';
import { RequestPayload } from '../@types';
import logger from '../config/logger';

export const validator =
  (requestPayload: RequestPayload) =>
  async (req: Request, res: Response, next: NextFunction): Promise<void> => {
    try {
      if (requestPayload.body) await requestPayload.body.validate(req.body);
      if (requestPayload.query) await requestPayload.query.validate(req.query);
      if (requestPayload.params) await requestPayload.params.validate(req.params);
      next();
    } catch (error) {
      logger.error(error);
      next(boom.badRequest('Invalid request payload'));
    }
  };
