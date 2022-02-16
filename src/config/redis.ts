import { createClient } from 'redis';
import logger from './logger';

const redisClient = createClient({ url: 'redis://redis:6379' });

redisClient.on('error', err => {
  logger.error(err);
  redisClient.quit();
});

redisClient.on('ready', () => {
  logger.info('Redis is ready');
});

redisClient.on('end', () => {
  logger.info('Redis terminated');
});

export default redisClient;
