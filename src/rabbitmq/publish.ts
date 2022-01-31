import amqp from 'amqplib';
import { QueueName } from '../@types';

const connectPublisher = async (): Promise<amqp.Channel> => {
  try {
    const connection = await amqp.connect('amqp://rabbitmq:5672');
    const channel = await connection.createChannel();

    await channel.assertQueue(QueueName.DOWNLOAD_TORRENT, { durable: false });
    await channel.assertQueue(QueueName.CONVERT_VIDEO, { durable: false });
    await channel.assertQueue(QueueName.MOVE_VIDEO, { durable: false });
    console.log('connected to rabbitmq publisher');
    return channel;
  } catch (error) {
    console.error(error);
    throw new Error('Error connecting to RabbitMQ');
  }
};

export default connectPublisher;
