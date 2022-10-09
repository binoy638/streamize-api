import { Router } from 'express';
import * as Yup from 'yup';
import * as watchPartyController from '../controllers/watchParty.controller';
import { getCurrentUser } from '../middlewares/currentUser.middleware';
import { validator } from '../middlewares/validator.middleware';

const watchPartyRouter = Router();

watchPartyRouter.post(
  '/create',
  getCurrentUser,
  validator({
    body: Yup.object().shape({
      maxViewers: Yup.number().required(),
      partyPlayerControl: Yup.boolean().required(),
    }),
  }),
  watchPartyController.create
);

watchPartyRouter.get(
  '/:slug',
  validator({
    params: Yup.object().shape({
      slug: Yup.string().min(5).max(5).required(),
    }),
  }),
  watchPartyController.getWatchParty
);

export default watchPartyRouter;
