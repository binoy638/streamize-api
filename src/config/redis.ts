import { createClient } from 'redis';

const redisClient = createClient({ url: 'redis://redis:6379' });

redisClient.on('error', err => {
  console.log({ redisError: err });
  redisClient.quit();
});

redisClient.on('ready', () => {
  console.log('Redis is ready');
});

redisClient.on('end', () => {
  console.log('Redis terminated');
});

export default redisClient;
