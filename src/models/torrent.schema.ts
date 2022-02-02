import { model, Schema } from 'mongoose';
import { nanoid } from 'nanoid';
import { ITorrent } from '../@types';

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
    files: [
      {
        name: { type: String },
        slug: { type: String, default: () => nanoid(5).toLowerCase() },
        path: { type: String },
        size: { type: Number },
        ext: { type: String },
        isConvertable: { type: String },
        status: {
          type: String,
          enum: ['downloading', 'paused', 'done', 'error', 'waiting', 'converting', 'added'],
        },
        convertStatus: {
          progress: { type: Number },
          state: { type: String, enum: ['processing', 'done', 'error', 'waiting'] },
        },
      },
    ],
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
    downloadInfo: {
      downloadSpeed: { type: Number },
      uploadSpeed: { type: Number },
      progress: { type: Number },
      timeRemaining: { type: Number },
      paused: { type: Boolean },
      completed: { type: Boolean },
    },
  },
  { timestamps: true }
);

export const TorrentModel = model<ITorrent>('Torrent', torrentSchema);
