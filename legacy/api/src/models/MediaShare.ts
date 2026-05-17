/* eslint-disable func-names */
import { model, Schema, Document } from 'mongoose';
import { nanoid } from 'nanoid';

interface MediaShareDoc extends Document {
  user: Schema.Types.ObjectId;
  slug: string;
  torrent: Schema.Types.ObjectId;
  mediaId: string;
  isTorrent: boolean;
  expiresIn: Schema.Types.Date;
}

const mediaShareSchema = new Schema<MediaShareDoc>({
  user: { type: Schema.Types.ObjectId, ref: 'User', required: true },
  slug: { type: String, default: () => nanoid(5).toLowerCase(), unique: true, required: true },
  torrent: { type: Schema.Types.ObjectId, ref: 'Torrent', required: true },
  mediaId: { type: String, required: true },
  isTorrent: { type: Boolean, required: true },
  expiresIn: { type: Date, required: true },
});

export const MediaShareModel = model<MediaShareDoc>('MediaShare', mediaShareSchema);
