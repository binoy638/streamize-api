import { Router } from 'express';
import * as Yup from 'yup';
import { validator } from '../middlewares/validator.middleware';
import * as authController from '../controllers/auth.controller';
import { RequestPayload } from '../@types';
import { getCurrentUser } from '../middlewares/currentUser.middleware';

const authRouter = Router();

const userValidator: RequestPayload = {
  body: Yup.object().shape({
    username: Yup.string().min(5).required(),
    password: Yup.string().min(5).required(),
    allocatedMemory: Yup.number().min(2000000000).required(),
  }),
};

const loginValidator: RequestPayload = {
  body: Yup.object().shape({
    username: Yup.string().min(5).required(),
    password: Yup.string().min(5).required(),
  }),
};

authRouter.post('/create', validator(userValidator), authController.createUser);

authRouter.post('/signin', validator(loginValidator), authController.signin);

authRouter.post('/verify', getCurrentUser, authController.verifyUser);

authRouter.post('/log', authController.log);

authRouter.post('/signout', authController.signout);

export default authRouter;
