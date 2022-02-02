import { Torrent } from 'webtorrent';
import { IDownloadInfo } from '../@types';

export const getDataFromTorrent = (torrent: Torrent): IDownloadInfo => {
  return {
    downloadSpeed: torrent.downloadSpeed,
    uploadSpeed: torrent.uploadSpeed,
    progress: torrent.progress,
    timeRemaining: torrent.timeRemaining,
    paused: torrent.paused,
    completed: torrent.done,
  };
};
