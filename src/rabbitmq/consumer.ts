/* eslint-disable unicorn/no-useless-undefined */
import amqp from 'amqplib';
import path from 'path';
import { ITorrent, IVideo, QueueName, TorrentPath } from '../@types';
import client from '../config/webtorrent';
import { allowedExt, convertableExt } from '../utils/misc';
import { updateNoMediaTorrent, updateTorrentInfo } from '../utils/query';
import { convertMKVtoMp4 } from '../utils/videoConverter';

// eslint-disable-next-line sonarjs/cognitive-complexity
const connectConsumer = async (rabbitMqPublisher: amqp.Channel): Promise<void> => {
  try {
    const connection = await amqp.connect('amqp://rabbitmq:5672');
    const channel = await connection.createChannel();
    channel.prefetch(5);
    await channel.assertQueue(QueueName.DOWNLOAD_TORRENT, { durable: false });
    await channel.assertQueue(QueueName.CONVERT_VIDEO, { durable: false });
    console.log('connected to rabbitmq consumer');

    channel.consume(QueueName.DOWNLOAD_TORRENT, message => {
      if (!message) return;
      console.log('Received new magnet link to process..');
      try {
        const addedTorrent: ITorrent = JSON.parse(message.content.toString());

        //! change path after convert
        client.add(addedTorrent.magnet, { path: TorrentPath.TMP }, async torrent => {
          const videofiles = torrent.files
            .map(file => {
              const ext = file.name.split('.').pop() || '';
              const isVideoFile = allowedExt.has(ext);
              const isConvertable = convertableExt.has(ext);
              if (!isVideoFile) {
                console.log("found file that's not video file");
                console.log(file.name);
                file.deselect();
              }
              return {
                name: file.name,
                path: file.path,
                size: file.length,
                ext,
                isConvertable,
                status: 'downloading',
              } as IVideo;
              // return addVideoFiles(addedTorrent._id, torrentfile);
            })
            .filter(file => allowedExt.has(file.ext));

          if (videofiles.length === 0) {
            torrent.destroy({ destroyStore: true });
            // eslint-disable-next-line no-underscore-dangle
            updateNoMediaTorrent(addedTorrent._id);
            channel.ack(message);
          } else {
            const { name, infoHash, length: size } = torrent;
            const isMultiVideos = videofiles.length > 1;
            const SavedTorrent = await updateTorrentInfo(addedTorrent._id, {
              name,
              infoHash,
              size,
              isMultiVideos,
              files: videofiles,
              status: 'downloading',
            });
            const convertableVideoFiles = SavedTorrent.files.filter(file => file && file?.isConvertable);
            if (convertableVideoFiles.length > 0) {
              torrent.on('done', () => {
                //* sending all convertable files to convert-video queue
                convertableVideoFiles.map(file =>
                  rabbitMqPublisher.sendToQueue(
                    QueueName.CONVERT_VIDEO,
                    Buffer.from(JSON.stringify({ torrentId: SavedTorrent._id, videofile: file }))
                  )
                );
                channel.ack(message);
                torrent.destroy();
              });
            }
          }
        });
      } catch (error) {
        console.log(error);
        channel.ack(message);
      }
    });

    channel.consume(QueueName.CONVERT_VIDEO, async message => {
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
          channel.ack(message);
        } else {
          console.log('inside not done');
          channel.ack(message);
        }
      } catch (error) {
        console.log(error);
        channel.ack(message);
      }
    });

    console.log('waiting for messages..');
  } catch (error) {
    console.error(error);
  }
};

export default connectConsumer;
