import { IVideo } from '.';

export interface IDeleteFilesMessageContent {
  src: string;
}

export interface IProcessVideoMessageContent extends IVideo {
  torrentID: string;
  torrentSlug: string;
}

export interface ITorrentDownloadStatusMessageContent {
  torrentID: string;
  torrentInfoHash: string;
}
