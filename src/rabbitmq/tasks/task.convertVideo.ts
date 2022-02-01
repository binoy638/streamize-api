import { ChannelWrapper } from 'amqp-connection-manager';
import { Channel, ConsumeMessage } from 'amqplib';
import path from 'path';
import { TorrentPath, QueueName } from '../../@types';
import { convertMKVtoMp4 } from '../../utils/videoConverter';

export const convertVideo =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    console.log('Received new video file to convert..');
    console.log(message.content.toString());
    const fileInfo = JSON.parse(message.content.toString());

    const { videofile, torrentId } = fileInfo;
    console.log(videofile);
    const fileNameWithoutExt = path.parse(videofile.name).name;
    const outputPath = `${TorrentPath.TMP}/${fileNameWithoutExt}.mp4`;
    try {
      const done = await convertMKVtoMp4(videofile.path, outputPath, videofile.slug, torrentId);
      if (done) {
        console.log('acknowledged');
        publisherChannel.sendToQueue(QueueName.FILE_MANAGER, JSON.stringify(videofile));
        channel.ack(message);
      } else {
        console.log('inside not done');
        channel.ack(message);
      }
    } catch (error) {
      console.log(error);
      channel.ack(message);
    }
  };
