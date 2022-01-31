import amqp from 'amqplib';
import { ITorrent } from '../@types';
import client from '../config/webtorrent';
import { allowedExt } from '../utils/misc';

const connectConsumer = async (rabbitMqPublisher: amqp.Channel): Promise<void> => {
  try {
    const connection = await amqp.connect('amqp://rabbitmq:5672');
    const channel = await connection.createChannel();
    channel.prefetch(5);
    await channel.assertQueue('download-torrent', { durable: false });
    await channel.assertQueue('convert-video', { durable: false });
    console.log('connected to rabbitmq consumer');
    channel.consume('download-torrent', message => {
      if (!message) return;
      console.log('Received new magnet link to process..');
      const addedTorrent: ITorrent = JSON.parse(message.content.toString());
      //! fix the path
      client.add(addedTorrent.magnet, { destroyStoreOnDestroy: true, path: 'src/tmp' }, torrent => {
        const videofiles = torrent.files
          .map(file => {
            const isVideoFile = allowedExt.has(file.name.split('.').pop() || '');
            if (!isVideoFile) {
              file.deselect();
            }
            return isVideoFile
              ? { name: file.name, path: file.path, size: file.length, ext: file.name.split('.').pop() || '' }
              : undefined;
          })
          .filter(file => file);
        if (videofiles.length === 0) {
          torrent.destroy({ destroyStore: true });
          channel.ack(message);
        } else {
          const convertableVideoFiles = videofiles.filter(file => file?.ext === 'mkv' || file?.ext === 'avi');
          if (convertableVideoFiles.length > 0) {
            torrent.on('done', () => {
              //* sending all convertable files to convert-video queue
              convertableVideoFiles.map(file =>
                rabbitMqPublisher.sendToQueue('convert-video', Buffer.from(JSON.stringify(file)))
              );
            });
          }
          channel.ack(message);
        }
      });
    });
    console.log('waiting for messages..');
  } catch (error) {
    console.error(error);
  }
};

export default connectConsumer;
