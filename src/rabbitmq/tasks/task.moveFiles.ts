import fs from 'fs-extra';
import { Channel, ConsumeMessage } from 'amqplib';
import path from 'path';
import { TorrentPath } from '../../@types';

export const moveFiles =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;
    console.log('Received new video file to move..');
    const file = JSON.parse(message.content.toString());
    const fileNameWithoutExt = path.parse(file.name).name;
    const dest = `${TorrentPath.DOWNLOAD}/${fileNameWithoutExt}.mp4`;
    const src = file.path;
    try {
      await fs.move(src, dest);
      console.log('file moved successfully!');
      channel.ack(message);
    } catch (error) {
      console.log(error);
      channel.ack(message);
    }
  };
