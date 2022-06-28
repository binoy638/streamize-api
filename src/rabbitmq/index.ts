/* eslint-disable func-names */
/* eslint-disable object-shorthand */
import amqp, { Channel } from 'amqp-connection-manager';
import { QueueName } from '../@types';
import logger from '../config/logger';
// eslint-disable-next-line import/namespace
import { deleteFiles } from './tasks/task.deleteFiles';
import { downloadTorrent } from './tasks/task.downloadtorrent';
import { generateSprite } from './tasks/task.generateSprite';
import { inspectVideo } from './tasks/task.inspectVideo';
import { processVideo } from './tasks/task.processVideo';

export const connection = amqp.connect(['amqp://rabbitmq:5672']);

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
      channel.assertQueue(QueueName.PROCESS_VIDEO_CPU_INTENSIVE, { durable: true }),
      channel.assertQueue(QueueName.PROCESS_VIDEO_NON_CPU_INTENSIVE, { durable: true }),
      channel.assertQueue(QueueName.INSPECT_VIDEO, { durable: true }),
      channel.assertQueue(QueueName.FILE_DELETE, { durable: true }),
    ]);
  },
});

export const torrentChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.assertQueue(QueueName.DOWNLOAD_TORRENT, { durable: true }),
      channel.consume(QueueName.DOWNLOAD_TORRENT, downloadTorrent(channel, publisherChannel), { noAck: true }),
    ]);
  },
});

export const videoInspectionChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(5),
      channel.assertQueue(QueueName.INSPECT_VIDEO, { durable: true }),
      channel.consume(QueueName.INSPECT_VIDEO, inspectVideo(channel, publisherChannel)),
    ]);
  },
});

export const cpuIntensiveVideoProcessingChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(1),
      channel.assertQueue(QueueName.PROCESS_VIDEO_CPU_INTENSIVE, { durable: true }),
      channel.consume(QueueName.PROCESS_VIDEO_CPU_INTENSIVE, processVideo(channel, publisherChannel)),
    ]);
  },
});

export const nonCpuIntensiveVideoProcessingChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(1),
      channel.assertQueue(QueueName.PROCESS_VIDEO_NON_CPU_INTENSIVE, { durable: true }),
      channel.consume(QueueName.PROCESS_VIDEO_NON_CPU_INTENSIVE, processVideo(channel, publisherChannel)),
    ]);
  },
});

export const spriteGenerationChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(1),
      channel.assertQueue(QueueName.GENERATE_SPRITE, { durable: true }),
      channel.consume(QueueName.GENERATE_SPRITE, generateSprite(channel)),
    ]);
  },
});

export const fileManagerChannel = connection.createChannel({
  json: true,
  setup: function (channel: Channel) {
    return Promise.all([
      channel.prefetch(1),
      channel.assertQueue(QueueName.FILE_DELETE, { durable: true }),
      channel.consume(QueueName.FILE_DELETE, deleteFiles(channel)),
    ]);
  },
});
