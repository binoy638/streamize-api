import { Router } from 'express';
import * as Yup from 'yup';
import * as mediaShareController from '../controllers/mediaShareController';
import { getCurrentUser } from '../middlewares/currentUser.middleware';
import { validator } from '../middlewares/validator.middleware';

const watchPartyRouter = Router();

watchPartyRouter.post(
  '/create',
  getCurrentUser,
  validator({
    body: Yup.object().shape({
      host: Yup.string().required(),
      maxViewers: Yup.number().required(),
      partyPlayerControl: Yup.boolean().required(),
    }),
  }),
  mediaShareController.create
);

export default watchPartyRouter;
