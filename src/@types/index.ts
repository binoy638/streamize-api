export interface IDownloadStatus {
  downloadSpeed: number;
  uploadSpeed: number;
  progress: number;
  paused: boolean;
  done: boolean;
}
export type ConvertState = 'processing' | 'done' | 'error' | 'waiting';

export interface IVideo {
  name: string;
  size: number;
  path: string;
  ext: string;
  isConvertable: boolean;
  downloadStatus: IDownloadStatus;
  convertStatus: {
    progress: number;
    state: ConvertState;
  };
}

export type TorrentState = 'added' | 'downloading' | 'converting' | 'done' | 'error' | 'waiting' | 'pause';

export interface ITorrent {
  _id: string;
  magnet: string;
  infoHash: string;
  name: string;
  size: number;
  files: IVideo[];
  isMultiVideos: boolean;
  downloadStatus: IDownloadStatus;
  isMedia: boolean;
}
