import { Torrent, TorrentFile } from 'webtorrent';
import { IDownloadInfo, IFileDownloadInfo } from '../@types';

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

export const getDataFromTorrentFile = (file: TorrentFile): IFileDownloadInfo => {
  return {
    downloaded: file.downloaded,
    progress: file.progress,
  };
};
