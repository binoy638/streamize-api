/* eslint-disable max-classes-per-file */
/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable class-methods-use-this */
import { ObjectType, Field, ID, Int, registerEnumType, Float, InputType } from 'type-graphql';
import { Video } from '../Video/video.types';

enum TorrentState {
  DOWNLOADING = 'downloading',
  PAUSED = 'paused',
  DONE = 'done',
  QUEUED = 'queued',
  ERROR = 'error',
  ADDED = 'added',
  PROCESSING = 'processing',
}

registerEnumType(TorrentState, {
  name: 'TorrentState',
});

@ObjectType()
class DownloadInfo {
  @Field(type => Int)
  downloadSpeed!: number;

  @Field(type => Int)
  uploadSpeed!: number;

  @Field(type => Float)
  progress!: number;

  @Field(type => Int)
  timeRemaining!: number;

  @Field(type => Int)
  paused!: boolean;

  @Field(type => Int)
  completed!: boolean;
}

@ObjectType()
export class Torrent {
  @Field(type => ID)
  id!: string;

  @Field()
  slug!: string;

  @Field()
  magnet!: string;

  @Field()
  infoHash?: string;

  @Field()
  name?: string;

  @Field(type => Int)
  size?: number;

  @Field(type => [Video])
  files!: Video[];

  @Field(type => TorrentState)
  status!: TorrentState;

  @Field(type => DownloadInfo)
  downloadInfo?: DownloadInfo;
}
