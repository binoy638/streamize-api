import { ITorrent, IVideo, UserPayload } from './index';

export interface IDeleteFilesMessageContent {
  src: string;
}

export interface IProcessVideoMessageContent extends IVideo {
  torrentID: string;
  torrentSlug: string;
}

export interface ITorrentDownloadMessageContent {
  torrent: ITorrent;
  currentUser: UserPayload;
}

export interface ISpriteGenerationMessageContent extends IProcessVideoMessageContent {
  inputPath: string;
}
