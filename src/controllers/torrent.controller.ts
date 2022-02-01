import { Request, Response } from 'express';
import boom from '@hapi/boom';
import 'express-async-errors';
import { getDataFromTorrent } from '../utils/torrentHelper';
import client from '../config/webtorrent';
import { createTorrentWithMagnet } from '../utils/query';
import { publisherChannel } from '../rabbitmq';

export const getAllTorrents = async (req: Request, res: Response): Promise<void> => {
  const torrents = client.torrents.map(torrent => torrent.infoHash);

  res.send({ torrents });
};

export const getTorrentInfoFromId = async (req: Request, res: Response): Promise<void> => {
  const { id } = req.params;
  const torrent = client.get(id);
  if (torrent) {
    res.send(getDataFromTorrent(torrent));
  } else {
    res.status(404).send({ message: 'torrent not found with given id' });
  }
};

export const deleteTorrentFromId = async (req: Request, res: Response): Promise<void> => {
  const { id } = req.params;

  const torrent = client.get(id);
  if (torrent) {
    torrent.destroy({}, () => {
      res.send({ message: 'torrent deleted' });
    });
  } else {
    res.status(404).send({ message: 'torrent not found' });
  }
};

// {
//   path: path.join(__dirname, '../../public/torrents');
// }

export const addTorrent = async (req: Request, res: Response): Promise<void> => {
  const { magnet } = req.body;
  if (!magnet) {
    throw boom.badRequest('magnet link is required');
  }

  const exist = client.get(magnet);
  if (exist) {
    throw boom.badRequest('torrent already exist');
  }
  try {
    const torrent = await createTorrentWithMagnet(magnet);

    publisherChannel
      .sendToQueue('download-torrent', torrent)
      .then(() => {
        res.send({ message: 'torrent added successfully', torrent });
      })
      .catch(error => {
        console.log(error);
        throw boom.internal('something went wrong');
      });
  } catch (error) {
    console.error(error);
    throw boom.internal('something went wrong');
  }
};

// export const addTorrent = async (req: Request, res: Response): Promise<void> => {
//   try {
//     const { magnet } = req.body;

//     client.add(magnet, { destroyStoreOnDestroy: true, path: 'src/tmp' }, torrent => {
//       const videofiles = torrent.files
//         .map(file => {
//           const isVideoFile = allowedExt.has(file.name.split('.').pop() || '');
//           if (!isVideoFile) {
//             file.deselect();
//           }
//           return isVideoFile
//             ? { name: file.name, path: file.path, size: file.length, ext: file.name.split('.').pop() || '' }
//             : undefined;
//         })
//         .filter(file => file);
//       if (videofiles.length === 0) {
//         torrent.destroy({ destroyStore: true });
//         res.status(400).send({ message: 'no video files found' });
//       } else {
//         const convertableVideoFiles = videofiles.filter(file => file?.ext === 'mkv' || file?.ext === 'avi');
//         if (convertableVideoFiles.length > 0) {
//           torrent.on('done', () => {
//             // TODO: need to use rabbitmq here
//             // const fileNameWithoutExt = path.parse(mkvfile.name).name;
//             // convertMKVtoMp4(
//             //   mkvfile.path,
//             //   `/home/app/src/videos/${fileNameWithoutExt}.mp4`,
//             //   torrent.infoHash,
//             //   fileNameWithoutExt
//             // ).catch(error => console.log(error));
//           });
//         }
//         res.send({
//           message: 'Torrent is downloading',
//           InfoHash: torrent.infoHash,
//           files,
//           convertable,
//         });
//       }

//       // {
//       //         return { name: file.name, path: file.path, size: file.length, ext: file.name.split('.').pop() || '' };
//       //       }
//     });
//   } catch (error) {
//     console.log({ error });
//   }
// };
