/* eslint-disable func-names */
import { Model, model, Schema } from 'mongoose';
import { nanoid } from 'nanoid';
import { ITorrent, ISubtitle, IVideo, VideoState, TorrentState } from '../@types';
import logger from '../config/logger';

interface ITorrentModel extends Model<ITorrent> {
  getTorrents(): Promise<ITorrent[]>;
}

const subtitleSchema: Schema = new Schema<ISubtitle>({
  fileName: String,
  title: String,
  language: String,
  path: String,
});

const fileSchema: Schema = new Schema<IVideo>({
  name: { type: String },
  slug: { type: String, default: () => nanoid(5).toLowerCase() },
  path: { type: String },
  size: { type: Number },
  ext: { type: String },
  subtitles: { type: [subtitleSchema], default: [] },
  status: {
    type: String,
    enum: [VideoState.DOWNLOADING, VideoState.PROCESSING, VideoState.DONE, VideoState.ERROR, VideoState.QUEUED],
  },
  transcodingPercent: {
    type: Number,
    default: 0,
  },
});

const torrentSchema = new Schema<ITorrent, ITorrentModel>(
  {
    slug: { type: String, default: () => nanoid(5).toLowerCase() },
    magnet: {
      type: String,
      required: true,
    },
    infoHash: { type: String },
    name: {
      type: String,
    },
    size: {
      type: Number,
    },
    files: {
      type: [fileSchema],
      default: [],
    },
    status: {
      enum: [
        TorrentState.DOWNLOADING,
        TorrentState.PAUSED,
        TorrentState.DONE,
        TorrentState.QUEUED,
        TorrentState.ERROR,
        TorrentState.ADDED,
        TorrentState.PROCESSING,
      ],
    },
  },
  { timestamps: true }
);

//* Helper methods

torrentSchema.static('getTorrents', async function (): Promise<ITorrent[]> {
  try {
    const docs = await this.find({}).select('-files').limit(20).lean(true);
    if (!docs) return [];
    return docs;
  } catch (error) {
    logger.error(error);
    throw new Error('something went wrong while fetching all torrents');
  }
});

export const TorrentModel = model<ITorrent, ITorrentModel>('Torrent', torrentSchema);
