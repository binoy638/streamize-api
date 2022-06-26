/* eslint-disable sonarjs/prefer-immediate-return */
/* eslint-disable @typescript-eslint/explicit-module-boundary-types */
/* eslint-disable max-classes-per-file */

/* eslint-disable class-methods-use-this */
import { Resolver, Query, Arg, InputType, Field, Ctx } from 'type-graphql';
import { ITorrent, UserPayload } from '../../@types';
import { TorrentModel } from '../../models/torrent.schema';
import { UserModel } from '../../models/user.schema';
import Utils from '../../utils';
import { DiskUsage, Torrent, Video } from '../typeDefs/typeDefs';

@Resolver()
export class TorrentResolver {
  @Query(() => [Torrent])
  //! pagination
  async torrents(@Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const userDoc = await UserModel.findOne({ __id: ctx.user.id })
      .populate<{ torrents: ITorrent[] }>('torrents')
      .lean();
    if (!userDoc) throw new Error('User not found');
    return userDoc.torrents;
  }

  @Query(() => Torrent, { nullable: true })
  async torrent(@Arg('slug') slug: string, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const torrent = await TorrentModel.findOne({ slug }).lean();
    if (!torrent) throw new Error('Torrent not found');
    const haveAccess = await UserModel.find({ _id: ctx.user.id, torrents: { $in: [torrent._id] } });
    if (!haveAccess) throw new Error('Unauthorized');
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
  async video(@Arg('input') input: VideoInput, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const torrent = await TorrentModel.findOne({ slug: input.torrentSlug }).lean();
    if (!torrent) throw new Error('Video not found');

    const haveAccess = await UserModel.find({ _id: ctx.user.id, torrents: { $in: [torrent._id] } });

    if (!haveAccess) throw new Error('Unauthorized');

    return torrent.files.find(file => file.slug === input.videoSlug);
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
