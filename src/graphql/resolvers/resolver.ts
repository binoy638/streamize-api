/* eslint-disable sonarjs/prefer-immediate-return */
/* eslint-disable @typescript-eslint/explicit-module-boundary-types */
/* eslint-disable max-classes-per-file */

/* eslint-disable class-methods-use-this */
import { Resolver, Query, Arg, InputType, Field, Ctx } from 'type-graphql';
import { UserPayload } from '../../@types';
import { TorrentModel } from '../../models/torrent.schema';
import Utils from '../../utils';
import { DiskUsage, Torrent, Video } from '../typeDefs/typeDefs';

@Resolver()
export class TorrentResolver {
  @Query(() => [Torrent])
  //! pagination
  async torrents() {
    const torrents = await TorrentModel.find({}).lean();
    return torrents;
  }

  @Query(() => Torrent, { nullable: true })
  async torrent(@Arg('slug') slug: string) {
    const torrent = await TorrentModel.findOne({ slug }).lean();
    if (!torrent) throw new Error('Torrent not found');
    return torrent;
  }
}

@InputType()
class VideoInput {
  @Field()
  torrentSlug!: string;

  @Field()
  videoSlug!: string;
}

@Resolver()
export class VideoResolver {
  @Query(() => Video, { nullable: true })
  async video(@Arg('input') input: VideoInput) {
    const video = await TorrentModel.findOne({ slug: input.torrentSlug }).lean();
    if (!video) throw new Error('Video not found');

    return video.files.find(file => file.slug === input.videoSlug);
  }
}

@Resolver()
export class UserResolver {
  @Query(() => DiskUsage)
  async diskUsage(@Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const usage = await Utils.getUserDiskUsage(ctx.user);
    return usage;
  }
}
