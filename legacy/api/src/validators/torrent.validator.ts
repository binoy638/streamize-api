import * as Yup from 'yup';
import { RequestPayload } from '../@types';

const magnetRegex = /^magnet:\?xt=urn:btih:[\dA-Fa-f]{40,}.*$/;

export const torrentSlugValidator: RequestPayload = {
  params: Yup.object().shape({
    slug: Yup.string().min(5).max(5).required(),
  }),
  body: undefined,
  query: undefined,
};

export const torrentMagnetValidator: RequestPayload = {
  body: Yup.object().shape({
    magnet: Yup.string().matches(magnetRegex, 'Invalid magnet link').required(),
  }),
  params: undefined,
  query: undefined,
};
