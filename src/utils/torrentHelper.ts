import { Torrent } from 'webtorrent';
import prettyBytes from 'pretty-bytes';

interface TorrentInfo {
  downloadSpeed: string;
  uploadSpeed: string;
  progress: string;
  paused: boolean;
  done: boolean;
}

export const getDataFromTorrent = (torrent: Torrent): TorrentInfo => {
  return {
    downloadSpeed: prettyBytes(torrent.downloadSpeed),
    uploadSpeed: prettyBytes(torrent.uploadSpeed),
    progress: `${torrent.progress * 100} %`,
    paused: torrent.paused,
    done: torrent.done,
  };
};
