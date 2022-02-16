import mongoose from 'mongoose';
import logger from './logger';

const connectMongo = async (): Promise<void> => {
  try {
    const connection = await mongoose.connect('mongodb://mongo:27017/cloud-torrent');

    if (connection) {
      logger.info(`connected to mongo`);
    }
  } catch (error) {
    logger.error(error);
    throw new Error('Error connecting to MongoDB');
  }
};

export default connectMongo;
