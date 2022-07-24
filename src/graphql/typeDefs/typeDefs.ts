/* eslint-disable max-classes-per-file */
/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable class-methods-use-this */
import { ObjectType, Field, ID, registerEnumType, Float, createUnionType } from 'type-graphql';
import { TorrentState, VideoState } from '../../@types';

registerEnumType(TorrentState, {
  name: 'TorrentState',
});

registerEnumType(VideoState, {
  name: 'VideoState',
});

@ObjectType()
class VideoDownloadInfo {
  @Field()
  downloaded!: number;

  @Field()
  progress!: number;
}

@ObjectType()
class Subtitles {
  @Field(type => ID)
  _id!: string;

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
  _id!: string;

  @Field()
  slug!: string;

  @Field()
  name!: string;

  @Field()
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

  @Field(() => VideoDownloadInfo, { nullable: true })
  downloadInfo?: VideoDownloadInfo;

  @Field(type => Float)
  transcodingPercent!: number;
}

@ObjectType()
export class DownloadInfo {
  @Field()
  downloadSpeed!: number;

  @Field()
  uploadSpeed!: number;

  @Field()
  progress!: number;

  @Field()
  timeRemaining!: number;

  @Field()
  paused!: boolean;

  @Field()
  completed!: boolean;
}

@ObjectType()
export class TorrentWithoutInfo {
  @Field(type => TorrentState)
  status!: TorrentState.ADDED | TorrentState.ERROR;

  @Field(type => ID)
  _id!: string;

  @Field()
  slug!: string;

  @Field()
  magnet!: string;
}

@ObjectType()
export class TorrentWithInfo {
  @Field(type => TorrentState)
  status!: TorrentState.DOWNLOADING | TorrentState.PAUSED | TorrentState.PROCESSING | TorrentState.QUEUED;

  @Field(type => ID)
  _id!: string;

  @Field()
  slug!: string;

  @Field()
  magnet!: string;

  @Field()
  infoHash!: string;

  @Field()
  name!: string;

  @Field()
  size!: number;

  @Field(type => [Video])
  files!: Video[];

  @Field(type => DownloadInfo)
  downloadInfo!: DownloadInfo;
}

// @ObjectType()
// export class Torrent {
//   @Field(type => ID)
//   _id!: string;

//   @Field()
//   slug!: string;

//   @Field()
//   magnet!: string;

//   @Field({ nullable: true })
//   infoHash?: string;

//   @Field({ nullable: true })
//   name?: string;

//   @Field({ nullable: true })
//   size?: number;

//   @Field(type => [Video])
//   files!: Video[];

//   @Field(type => TorrentState)
//   status!: TorrentState;

//   @Field(type => DownloadInfo, { nullable: true })
//   downloadInfo?: DownloadInfo;
// }

export const Torrent = createUnionType({
  name: 'Torrent', // the name of the GraphQL union
  types: () => [TorrentWithInfo, TorrentWithoutInfo] as const,
  resolveType: value => {
    if ('name' in value) {
      return TorrentWithInfo; // we can return object type class (the one with `@ObjectType()`)
    }

    return TorrentWithoutInfo;
  },
});

@ObjectType()
export class DiskUsage {
  @Field()
  size!: number;

  @Field()
  free!: number;
}

@ObjectType()
export class User {
  @Field(type => ID)
  _id!: string;

  @Field(type => ID)
  username!: string;

  @Field()
  torrents!: string;

  @Field()
  allocatedMemory!: number;

  @Field()
  isAdmin!: boolean;
}

@ObjectType()
export class SharedPlaylist {
  @Field(type => ID)
  _id!: string;

  @Field()
  slug!: string;

  @Field(() => User)
  user!: User;

  @Field(() => Torrent)
  torrent!: typeof Torrent;

  @Field()
  mediaId!: string;

  @Field()
  isTorrent!: boolean;

  @Field()
  expiresIn!: Date;
}
