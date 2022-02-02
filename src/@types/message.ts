import { IVideo } from '.';

export interface IMoveFilesMessageContent {
  src: string;
  dest: string;
  torrentID: string;
  fileSlug: string;
}

export interface IDeleteFilesMessageContent {
  src: string;
}

export interface IConvertVideoMessageContent extends IVideo {
  torrentID: string;
}

export interface ITorrentDownloadStatusMessageContent {
  torrentID: string;
  torrentInfoHash: string;
}
