import mongoose from 'mongoose';

const connectMongo = async (): Promise<void> => {
  try {
    const connection = await mongoose.connect('mongodb://mongo:27017/cloud-torrent');

    if (connection) {
      console.log('connected to mongo');
    }
  } catch (error) {
    console.log(error);
    throw new Error('Error connecting to MongoDB');
  }
};

export default connectMongo;
