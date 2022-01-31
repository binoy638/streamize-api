import { model, Schema } from 'mongoose';
import { ITorrent } from '../@types';

const torrentSchema: Schema = new Schema<ITorrent>(
  {
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
        path: { type: String },
        size: { type: Number },
        ext: { type: String },
        isConvertable: { type: String },
        downloadStatus: { type: Number },
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
    },
    downloadStatus: {
      type: String,
      enum: ['downloading', 'paused', 'done', 'error', 'waiting', 'converting', 'added'],
    },
  },
  { timestamps: true }
);

export const TorrentModel = model<ITorrent>('Torrent', torrentSchema);
