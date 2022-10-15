/* eslint-disable func-names */
import { model, Schema, Document } from 'mongoose';
import { nanoid } from 'nanoid';

interface WatchPartyDoc extends Document {
  host: Schema.Types.ObjectId;
  name: string;
  slug: string;
  partyPlayerControl: boolean;
  maxViewers: number;
}

const watchPartySchema = new Schema<WatchPartyDoc>(
  {
    host: { type: Schema.Types.ObjectId, ref: 'User', required: true },
    name: { type: String, required: true },
    slug: { type: String, default: () => nanoid(5).toLowerCase(), unique: true, required: true },
    partyPlayerControl: { type: Boolean, default: false },
    maxViewers: { type: Number, default: 0 },
  },
  {
    timestamps: true,
  }
);

export const WatchPartyModel = model<WatchPartyDoc>('WatchParty', watchPartySchema);
