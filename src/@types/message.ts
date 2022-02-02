import { IVideo } from '.';

export interface IMoveFilesMessageContent {
  src: string;
  dest: string;
}

export interface IDeleteFilesMessageContent {
  src: string;
}

export interface IConvertVideoMessageContent extends IVideo {
  torrentID: string;
}
