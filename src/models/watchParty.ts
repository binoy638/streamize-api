/* eslint-disable func-names */
import { model, Schema, Document } from 'mongoose';
import { nanoid } from 'nanoid';

interface WatchPartyDoc extends Document {
  host: Schema.Types.ObjectId;
  slug: string;
  currentVideo: { torrent: Schema.Types.ObjectId; video: string };
  viewers: string[];
  partyPlayerControl: boolean;
  maxViewers: number;
}

const watchPartySchema = new Schema<WatchPartyDoc>(
  {
    host: { type: Schema.Types.ObjectId, ref: 'User', required: true },
    slug: { type: String, default: () => nanoid(5).toLowerCase(), unique: true, required: true },
    currentVideo: { type: { torrent: { type: Schema.Types.ObjectId }, video: { type: String } } },
    viewers: { type: [String], default: [] },
    partyPlayerControl: { type: Boolean, default: false },
    maxViewers: { type: Number, default: 0 },
  },
  {
    timestamps: true,
  }
);

export const WatchPartyModel = model<WatchPartyDoc>('WatchParty', watchPartySchema);
