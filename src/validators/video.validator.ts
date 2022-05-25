import * as Yup from 'yup';
import { RequestPayload } from '../@types';

export const streamVideoValidator: RequestPayload = {
  params: Yup.object().shape({
    videoSlug: Yup.string().min(5).max(5).required(),
    filename: Yup.string().required(),
  }),
  body: undefined,
  query: undefined,
};
