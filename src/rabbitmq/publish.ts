import amqp from 'amqplib';

const connectPublisher = async (): Promise<amqp.Channel> => {
  try {
    const connection = await amqp.connect('amqp://rabbitmq:5672');
    const channel = await connection.createChannel();

    await channel.assertQueue('download-torrent', { durable: false });
    await channel.assertQueue('convert-video', { durable: false });
    console.log('connected to rabbitmq publisher');
    return channel;
  } catch (error) {
    console.error(error);
    throw new Error('Error connecting to RabbitMQ');
  }
};

export default connectPublisher;
