import { model, Schema } from 'mongoose';
import { nanoid } from 'nanoid';
import { ITorrent, ISubtitle, IVideo } from '../@types';

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
  isConvertable: { type: Boolean, default: false },
  status: {
    type: String,
    enum: ['downloading', 'paused', 'done', 'error', 'waiting', 'converting', 'added'],
  },
  convertStatus: {
    progress: { type: Number },
    state: { type: String, enum: ['processing', 'done', 'error', 'waiting'] },
  },
});

const torrentSchema: Schema = new Schema<ITorrent>(
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
    isMultiVideos: {
      type: Boolean,
    },
    isMedia: {
      type: Boolean,
      default: true,
    },
    status: {
      type: String,
      enum: ['downloading', 'paused', 'done', 'error', 'waiting', 'converting', 'added'],
    },
  },
  { timestamps: true }
);

export const TorrentModel = model<ITorrent>('Torrent', torrentSchema);
