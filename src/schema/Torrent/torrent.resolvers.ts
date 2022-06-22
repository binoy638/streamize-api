/* eslint-disable sonarjs/prefer-immediate-return */
/* eslint-disable @typescript-eslint/explicit-module-boundary-types */
/* eslint-disable max-classes-per-file */

/* eslint-disable class-methods-use-this */
import { Resolver, Query, Arg } from 'type-graphql';
import { TorrentModel } from '../../models/torrent.schema';
import { Torrent } from './torrent.types';

@Resolver()
export class TorrentResolver {
  @Query(() => [Torrent])
  async torrents() {
    const torrents = await TorrentModel.find({}).lean();
    return torrents;
  }

  @Query(() => Torrent, { nullable: true })
  async torrent(@Arg('slug') slug: string) {
    const torrent = await TorrentModel.findOne({ slug }).lean();
    return torrent;
  }
}
