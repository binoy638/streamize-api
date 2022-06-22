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
