/* eslint-disable func-names */
import { model, Schema, Document } from 'mongoose';

interface UserVideoProgressDoc extends Document {
  user: Schema.Types.ObjectId;
  video: string;
  progress: number;
}

const userVideoProgressSchema = new Schema<UserVideoProgressDoc>({
  user: { type: Schema.Types.ObjectId, ref: 'User', required: true },
  video: { type: String, required: true },
  progress: { type: Number, required: true, default: 0 },
});

export const UserVideoProgressModel = model<UserVideoProgressDoc>('UserVideoProgress', userVideoProgressSchema);
