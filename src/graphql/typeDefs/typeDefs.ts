/* eslint-disable max-classes-per-file */
/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable class-methods-use-this */
import { ObjectType, Field, ID, Int, registerEnumType, Float } from 'type-graphql';

enum VideoState {
  DOWNLOADING = 'downloading',
  PROCESSING = 'processing',
  DONE = 'done',
  ERROR = 'error',
  QUEUED = 'queued',
}

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

registerEnumType(VideoState, {
  name: 'VideoState',
});

@ObjectType()
class VideoDownloadInfo {
  @Field(() => Int)
  downloaded!: number;

  @Field(() => Float)
  progress!: number;
}

@ObjectType()
class Subtitles {
  @Field()
  fileName!: string;

  @Field()
  title!: string;

  @Field()
  language!: string;

  @Field()
  path!: string;
}

@ObjectType()
export class Video {
  @Field(type => ID)
  id!: string;

  @Field()
  slug!: string;

  @Field()
  name!: string;

  @Field(type => Int)
  size!: number;

  @Field()
  path!: string;

  @Field()
  ext!: string;

  @Field()
  progressPreview!: boolean;

  @Field(() => [Subtitles])
  subtitles!: Subtitles[];

  @Field(type => VideoState)
  status!: VideoState;

  @Field(() => VideoDownloadInfo)
  downloadInfo?: VideoDownloadInfo;
}

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

@ObjectType()
export class DiskUsage {
  @Field(type => Int)
  size!: number;

  @Field(type => Int)
  free!: number;
}

@ObjectType()
export class User {
  @Field(type => ID)
  username!: string;

  @Field()
  torrents!: string;

  @Field(type => Int)
  allocatedMemory!: number;

  @Field()
  isAdmin!: boolean;
}
