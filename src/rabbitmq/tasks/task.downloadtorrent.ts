import { Channel, ChannelWrapper } from 'amqp-connection-manager';
import { ConsumeMessage } from 'amqplib';
import { ITorrent, TorrentPath, IVideo, QueueName } from '../../@types';
import client from '../../config/webtorrent';
import { allowedExt, convertableExt } from '../../utils/misc';
import { updateNoMediaTorrent, updateTorrentInfo } from '../../utils/query';

export const downloadTorrent =
  (channel: Channel, publisherChannel: ChannelWrapper) =>
  async (message: ConsumeMessage | null): Promise<void> => {
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
          console.log('no video files found torrent destroyed');
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
          torrent.on('done', async () => {
            const convertableVideoFiles = SavedTorrent.files.filter(file => file.isConvertable);
            const nonConvertableVideoFiles = SavedTorrent.files.filter(file => !file.isConvertable);
            if (convertableVideoFiles.length > 0) {
              //* sending all convertable files to convert-video queue
              convertableVideoFiles.map(file =>
                publisherChannel.sendToQueue(QueueName.CONVERT_VIDEO, { torrentId: SavedTorrent._id, videofile: file })
              );
            }
            await Promise.all(
              nonConvertableVideoFiles.map(file => publisherChannel.sendToQueue(QueueName.FILE_MANAGER, { file }))
            );
            channel.ack(message);
            console.log('download complete torrent destroyed');
            torrent.destroy();
          });
        }
      });
    } catch (error) {
      console.log(error);
      channel.ack(message);
    }
  };
