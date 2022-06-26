/* eslint-disable sonarjs/prefer-immediate-return */
/* eslint-disable @typescript-eslint/explicit-module-boundary-types */
/* eslint-disable max-classes-per-file */

/* eslint-disable class-methods-use-this */
import { Resolver, Query, Arg, InputType, Field, Ctx, Int } from 'type-graphql';
import { ITorrent, TorrentState, UserPayload } from '../../@types';
import client from '../../config/webtorrent';
import { TorrentModel } from '../../models/torrent.schema';
import { UserModel } from '../../models/user.schema';
import { UserVideoProgressModel } from '../../models/userVideoProgress.schema';
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

    const { torrents } = userDoc;
    const torrentsWithDownloadInfo = torrents.map(torrent => {
      if (torrent.status === TorrentState.DOWNLOADING) {
        const torrentInClient = client.get(torrent.infoHash);
        if (torrentInClient) {
          return { ...torrent, downloadInfo: Utils.getDataFromTorrent(torrentInClient) };
        }
        return torrent;
      }
      return torrent;
    });
    return torrentsWithDownloadInfo;
  }

  @Query(() => Torrent)
  async torrent(@Arg('slug') slug: string, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const torrentDoc = await TorrentModel.findOne({ slug }).lean();
    if (!torrentDoc) throw new Error('Torrent not found');
    const haveAccess = await UserModel.find({ _id: ctx.user.id, torrents: { $in: [torrentDoc._id] } });
    if (!haveAccess) throw new Error('Unauthorized');
    if (torrentDoc.status === TorrentState.DOWNLOADING) {
      const torrent = client.get(torrentDoc.infoHash);
      if (torrent) {
        const downloadInfo = Utils.getDataFromTorrent(torrent);

        const filesWithDownloadInfo = torrentDoc.files.map(docFile => {
          const file = torrent.files.find(file => file.name === docFile.name);
          if (file) {
            const downloadInfo = Utils.getDataFromTorrentFile(file);
            return { ...docFile, downloadInfo };
          }
          return docFile;
        });
        torrentDoc.downloadInfo = downloadInfo;
        torrentDoc.files = filesWithDownloadInfo;
      }
    }
    return torrentDoc;
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
  @Query(() => Video)
  async video(@Arg('input') input: VideoInput, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const torrent = await TorrentModel.findOne({ slug: input.torrentSlug }).lean();
    if (!torrent) throw new Error('Video not found');

    const haveAccess = await UserModel.find({ _id: ctx.user.id, torrents: { $in: [torrent._id] } });

    if (!haveAccess) throw new Error('Unauthorized');

    const video = torrent.files.find(file => file.slug === input.videoSlug);
    if (!video) throw new Error('Video not found');
    return video;
  }

  @Query(() => Int)
  async videoProgress(@Arg('videoSlug') videoSlug: string, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new Error('User not found');
    const doc = await UserVideoProgressModel.findOne({ user: ctx.user.id, video: videoSlug });
    if (!doc) throw new Error('Video Progress not found');
    return doc.progress;
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