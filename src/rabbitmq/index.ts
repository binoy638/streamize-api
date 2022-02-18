/* eslint-disable func-names */
/* eslint-disable object-shorthand */
import amqp, { Channel } from 'amqp-connection-manager';
import { QueueName } from '../@types';
import logger from '../config/logger';
// eslint-disable-next-line import/namespace
import { convertVideo } from './tasks/task.convertVideo';
import { deleteFiles } from './tasks/task.deleteFiles';
import { downloadTorrent } from './tasks/task.downloadtorrent';
import { moveFiles } from './tasks/task.moveFiles';

const connection = amqp.connect(['amqp://rabbitmq:5672']);

connection.on('connect', function () {
  logger.info('Rabbitmq Connected!');
});
connection.on('disconnect', function (err) {
  logger.error(`Rabbitmq Disconnected. ${JSON.stringify(err)}`);
});

export const publisherChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.assertQueue(QueueName.DOWNLOAD_TORRENT, { durable: true }),
      channel.assertQueue(QueueName.CONVERT_VIDEO, { durable: true }),
      channel.assertQueue(QueueName.FILE_MOVE, { durable: true }),
      channel.assertQueue(QueueName.FILE_DELETE, { durable: true }),
    ]);
  },
});

export const torrentChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(5),
      channel.assertQueue(QueueName.DOWNLOAD_TORRENT, { durable: true }),
      channel.consume(QueueName.DOWNLOAD_TORRENT, downloadTorrent(channel, publisherChannel)),
    ]);
  },
});

export const videoChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(1),
      channel.assertQueue(QueueName.CONVERT_VIDEO, { durable: true }),
      channel.consume(QueueName.CONVERT_VIDEO, convertVideo(channel, publisherChannel)),
    ]);
  },
});

export const fileManagerChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(1),
      channel.assertQueue(QueueName.FILE_MOVE, { durable: true }),
      channel.assertQueue(QueueName.FILE_DELETE, { durable: true }),
      channel.consume(QueueName.FILE_MOVE, moveFiles(channel)),
      channel.consume(QueueName.FILE_DELETE, deleteFiles(channel)),
    ]);
  },
});
