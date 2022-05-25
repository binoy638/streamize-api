import { IVideo } from '.';

export interface IDeleteFilesMessageContent {
  src: string;
}

export interface IConvertVideoMessageContent extends IVideo {
  torrentID: string;
  torrentSlug: string;
}

export interface ITorrentDownloadStatusMessageContent {
  torrentID: string;
  torrentInfoHash: string;
}
