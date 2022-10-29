/* eslint-disable sonarjs/prefer-immediate-return */
/* eslint-disable @typescript-eslint/explicit-module-boundary-types */
/* eslint-disable max-classes-per-file */

/* eslint-disable class-methods-use-this */
import { AuthenticationError, ForbiddenError } from 'apollo-server-express';
import { Resolver, Query, Arg, InputType, Field, Ctx, Float, Mutation } from 'type-graphql';
import { ITorrent, TorrentState, UserPayload } from '../../@types';
import client from '../../config/webtorrent';
import { MediaShareModel } from '../../models/MediaShare';
import { TorrentModel } from '../../models/torrent.schema';
import { UserDoc, UserModel } from '../../models/user.schema';
import { UserVideoProgressModel } from '../../models/userVideoProgress.schema';
import { WatchPartyModel } from '../../models/watchParty';
import Utils from '../../utils';
import { DiskUsage, SharedPlaylist, Torrent, Video, WatchParty } from '../typeDefs/typeDefs';

@Resolver()
export class TorrentResolver {
  @Query(() => [Torrent])
  //! pagination
  async torrents(@Ctx() ctx: { user: UserPayload }) {
    console.log(ctx);
    if (!ctx.user) throw new AuthenticationError('User not found');
    const userDoc = await UserModel.findById(ctx.user.id)
      .sort({ createdAt: 1 })
      .populate<{ torrents: ITorrent[] }>({ path: 'torrents', options: { sort: { createdAt: -1 } } })
      .lean();
    if (!userDoc) throw new AuthenticationError('User not found');
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
    if (!ctx.user) throw new AuthenticationError('User not found');
    const torrentDoc = await TorrentModel.findOne({ slug }).lean();
    if (!torrentDoc) throw new Error('Torrent not found');
    const haveAccess = await UserModel.findOne({ _id: ctx.user.id, torrents: { $in: [torrentDoc._id] } });
    if (!haveAccess) throw new ForbiddenError('No access');
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

@InputType()
class SharedPlaylistVideoInput {
  @Field()
  slug!: string;

  @Field()
  videoSlug!: string;
}

@Resolver()
export class VideoResolver {
  @Query(() => Video)
  async video(@Arg('input') input: VideoInput, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new AuthenticationError('User not found');
    const torrent = await TorrentModel.findOne({ slug: input.torrentSlug }).lean();
    if (!torrent) throw new Error('Video not found');

    const haveAccess = await UserModel.find({ _id: ctx.user.id, torrents: { $in: [torrent._id] } });

    if (!haveAccess) throw new ForbiddenError('No access');

    const video = torrent.files.find(file => file.slug === input.videoSlug);
    if (!video) throw new Error('Video not found');
    return video;
  }

  @Query(() => Float)
  async videoProgress(@Arg('videoSlug') videoSlug: string, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new AuthenticationError('User not found');
    const doc = await UserVideoProgressModel.findOne({ user: ctx.user.id, video: videoSlug });
    if (!doc) return 0;
    return doc.progress;
  }
}

@Resolver()
export class UserResolver {
  @Query(() => DiskUsage)
  async diskUsage(@Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new AuthenticationError('User not found');
    //! temp fix
    const usage = await Utils.getUserDiskUsage(ctx.user);
    return usage;
  }
}

@Resolver()
export class SharedPlaylistResolver {
  @Query(() => SharedPlaylist)
  async sharedPlaylist(@Arg('slug') slug: string) {
    const playlist = await MediaShareModel.findOne({ slug, expiresIn: { $gt: new Date() } })
      .populate<{ torrent: ITorrent; user: UserDoc }>('torrent user')
      .lean();
    if (!playlist) throw new Error('Playlist not found');
    if (playlist.isTorrent) return playlist;
    const filteredTorrentFiles = playlist.torrent.files.filter(file => file.slug === playlist.mediaId);
    playlist.torrent.files = filteredTorrentFiles;
    return playlist;
  }

  @Query(() => Video)
  async sharedPlaylistVideo(@Arg('input') input: SharedPlaylistVideoInput) {
    const playlist = await MediaShareModel.findOne({ slug: input.slug, expiresIn: { $gt: new Date() } })
      .populate<{ torrent: ITorrent }>('torrent')
      .lean();
    if (!playlist) throw new Error('Playlist not found');

    //! check if playlist is torrent

    const video = playlist.torrent.files.find(file => file.slug === input.videoSlug);
    if (!video) throw new Error('Video not found');
    return video;
  }
}

@InputType({ description: 'New watch party data' })
class AddWatchPartyInput implements Partial<WatchParty> {
  @Field()
  name!: string;

  @Field()
  maxViewers!: number;

  @Field()
  partyPlayerControl!: boolean;
}

@InputType()
class WatchPartyVideoInput {
  @Field()
  watchPartySlug!: string;

  @Field()
  torrentSlug!: string;

  @Field()
  videoSlug!: string;
}

@Resolver()
export class WatchPartyResolver {
  @Query(() => WatchParty)
  async getWatchParty(@Arg('slug') slug: string) {
    const party = await WatchPartyModel.findOne({ slug }).populate<{ host: UserDoc }>('host').lean();
    if (!party) throw new Error('watch party not found');
    return party;
  }

  @Query(() => [WatchParty])
  async getUserWatchParties(@Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new AuthenticationError('User not found');
    const parties = await WatchPartyModel.find({ host: ctx.user.id }).populate<{ host: UserDoc }>('host').lean();

    return parties;
  }

  @Query(() => Video)
  async getWatchPartyVideo(@Arg('input') input: WatchPartyVideoInput) {
    const party = await WatchPartyModel.findOne({ slug: input.watchPartySlug });

    if (!party) throw new Error('Watch party not found');
    const torrent = await TorrentModel.findOne({ slug: input.torrentSlug }).lean();
    if (!torrent) throw new Error('Torrent not found');

    const haveAccess = await UserModel.find({ _id: party.host, torrents: { $in: [torrent._id] } });

    if (!haveAccess) throw new ForbiddenError('No access');

    const video = torrent.files.find(file => file.slug === input.videoSlug);
    if (!video) throw new Error('Video not found');
    return video;
  }

  @Mutation(() => WatchParty)
  async addWatchParty(@Arg('data') newWatchPartyData: AddWatchPartyInput, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new AuthenticationError('User not found');
    const { name, partyPlayerControl, maxViewers } = newWatchPartyData;

    const exists = await WatchPartyModel.findOne({ name });

    if (exists) throw new Error('Watch party with that name already exists');

    const newWatchParty = await WatchPartyModel.create({
      host: ctx.user.id,
      name,
      partyPlayerControl,
      maxViewers,
    });

    return newWatchParty;
  }

  @Mutation(() => Boolean)
  async removeWatchParty(@Arg('slug') slug: string, @Ctx() ctx: { user: UserPayload }) {
    if (!ctx.user) throw new AuthenticationError('User not found');

    const party = await WatchPartyModel.findOne({ slug, host: ctx.user.id });
    if (!party) throw new Error('Watch party not found');
    await party.remove();
    return true;
  }
}
