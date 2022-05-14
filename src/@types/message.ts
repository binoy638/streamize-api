import { IVideo } from '.';

export interface IMoveFilesMessageContent {
  src: string;
  dest: string;
  torrentID: string;
  torrentSlug: string;
  fileSlug: string;
}

export interface IDeleteFilesMessageContent {
  src: string;
  torrentSlug: string;
}

export interface IConvertVideoMessageContent extends IVideo {
  torrentID: string;
  torrentSlug: string;
}

export interface ITorrentDownloadStatusMessageContent {
  torrentID: string;
  torrentInfoHash: string;
}
