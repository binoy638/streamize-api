import { NextFunction, Request, Response } from 'express';
import boom from '@hapi/boom';
import jwt from 'jsonwebtoken';
import { UserModel } from '../models/user.schema';
import logger from '../config/logger';
import { PasswordManager } from '../libs/passwordManager';

export const createUser = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { username, password, allocatedMemory } = req.body;
  try {
    const user = new UserModel({ username, password, torrents: [], allocatedMemory });
    await user.save();
    res.status(201).send({ message: 'User created' });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const signin = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { username, password } = req.body;
  try {
    const user = await UserModel.findOne({ username });
    if (!user) {
      next(boom.unauthorized('Invalid username or password'));
      return;
    }
    const isValid = await PasswordManager.compare(password, user.password);
    if (!isValid) {
      next(boom.unauthorized('Invalid username or password'));
      return;
    }
    const JWTtoken = jwt.sign(
      { id: user._id, isAdmin: user.isAdmin, allocatedMemory: user.allocatedMemory },
      process.env.JWT_SECRET || ''
    );
    req.session = {
      jwt: JWTtoken,
    };
    res.status(200).send({ user: { username: user.username, torrents: user.torrents, isAdmin: user.isAdmin } });
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const signout = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  try {
    // eslint-disable-next-line unicorn/no-null
    req.session = undefined;
    res.send({});
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};

export const verifyUser = async (req: Request, res: Response, next: NextFunction): Promise<void> => {
  const { currentUser } = req;

  try {
    const user = await UserModel.findOne({ _id: currentUser.id }).lean();
    if (!user) {
      next(boom.unauthorized('User not found'));
      return;
    }
    res.sendStatus(200);
  } catch (error) {
    logger.error(error);
    next(boom.internal('Internal server error'));
  }
};
