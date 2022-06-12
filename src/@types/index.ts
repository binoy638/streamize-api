/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable no-shadow */
/* eslint-disable no-unused-vars */

import * as Yup from 'yup';

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

export enum VideoState {
  DOWNLOADING = 'downloading',
  PROCESSING = 'processing',
  DONE = 'done',
  ERROR = 'error',
  QUEUED = 'queued',
}

export enum TorrentState {
  DOWNLOADING = 'downloading',
  PAUSED = 'paused',
  DONE = 'done',
  QUEUED = 'queued',
  ERROR = 'error',
  ADDED = 'added',
  PROCESSING = 'processing',
}

export interface ISubtitle {
  fileName: string;
  title: string;
  language: string;
  path: string;
}

export interface IVideo {
  name: string;
  slug: string;
  size: number;
  path: string;
  ext: string;
  progressPreview: boolean;
  subtitles: ISubtitle[];
  status: VideoState;
  downloadInfo?: IFileDownloadInfo;
  transcodingPercent: number;
}

export interface ITorrent {
  _id: string;
  slug: string;
  magnet: string;
  infoHash: string;
  name: string;
  size: number;
  files: IVideo[];
  status: TorrentState;
  downloadInfo?: IDownloadInfo;
}

export enum TorrentPath {
  TMP = '/home/app/tmp',
  DOWNLOAD = '/home/app/downloads',
  SUBTITLES = '/home/app/subtitles',
}

export enum QueueName {
  DOWNLOAD_TORRENT = 'download-torrent',
  PROCESS_VIDEO_CPU_INTENSIVE = 'process-video-cpu-intensive',
  PROCESS_VIDEO_NON_CPU_INTENSIVE = 'process-video-non-cpu-intensive',
  INSPECT_VIDEO = 'inspect-video',
  FILE_DELETE = 'delete-files',
  GENERATE_SPRITE = 'generate-sprite',
}

export interface RequestPayload {
  body?: Yup.ObjectSchema<any> | undefined;
  query?: Yup.ObjectSchema<any> | undefined;
  params?: Yup.ObjectSchema<any> | undefined;
}

export interface UserPayload {
  id: string;
  isAdmin: boolean;
  username: boolean;
  allocatedMemory: number;
}
