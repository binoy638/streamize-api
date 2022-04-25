/* eslint-disable no-shadow */
/* eslint-disable no-unused-vars */
export interface IDownloadInfo {
  downloadSpeed: number;
  uploadSpeed: number;
  progress: number;
  timeRemaining: number;
  paused: boolean;
  completed: boolean;
}

export interface IFileDownloadInfo {
  downloaded: number;
  progress: number;
}

export type ConvertState = 'processing' | 'done' | 'error' | 'waiting';

export type TorrentStatus = 'added' | 'downloading' | 'converting' | 'done' | 'error' | 'waiting' | 'pause';

export interface ISubtitle {
  fileName: string;
  title: string;
  language: string;
  path: string;
}

export interface IVideo {
  // [x: string]: any;
  name: string;
  slug: string;
  size: number;
  path: string;
  ext: string;
  isConvertable: boolean;
  subtitles: ISubtitle[];
  status: TorrentStatus;
  downloadInfo?: IFileDownloadInfo;
  convertStatus?: {
    progress: number;
    state: ConvertState;
  };
}

export interface ITorrent {
  _id: string;
  slug: string;
  magnet: string;
  infoHash: string;
  name: string;
  size: number;
  files: IVideo[];
  isMultiVideos: boolean;
  status: TorrentStatus;
  isMedia: boolean;
  downloadInfo?: IDownloadInfo;
}

export enum TorrentPath {
  TMP = '/home/app/tmp',
  DOWNLOAD = '/home/app/downloads',
  SUBTITLES = '/home/app/subtitles',
}

export enum QueueName {
  DOWNLOAD_TORRENT = 'download-torrent',
  CONVERT_VIDEO = 'convert-video',
  FILE_MOVE = 'move-files',
  FILE_DELETE = 'delete-files',
}
