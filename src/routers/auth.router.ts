import { Router } from 'express';
import * as Yup from 'yup';
import { validator } from '../middlewares/validator.middleware';
import * as authController from '../controllers/auth.controller';
import { RequestPayload } from '../@types';

const authRouter = Router();

const userValidator: RequestPayload = {
  body: Yup.object().shape({
    username: Yup.string().min(5).required(),
    password: Yup.string().min(5).required(),
  }),
};

authRouter.post('/create', validator(userValidator), authController.createUser);

authRouter.post('/signin', validator(userValidator), authController.signin);

authRouter.post('/signout', authController.signout);

export default authRouter;
