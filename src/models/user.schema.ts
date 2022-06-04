/* eslint-disable no-param-reassign */
/* eslint-disable func-names */
import { model, Schema, Document } from 'mongoose';
import { PasswordManager } from '../libs/passwordManager';

interface UserDoc extends Document {
  username: string;
  password: string;
  torrents: unknown[];
  allocatedMemory: number;
  isAdmin: boolean;
}

const userSchema = new Schema<UserDoc>(
  {
    username: { type: String, required: true, unique: true },
    password: { type: String, required: true },
    torrents: [{ type: Schema.Types.ObjectId, ref: 'Torrent' }],
    allocatedMemory: { type: Number, required: true, default: 0 },
    isAdmin: { type: Boolean, required: true, default: false },
  },
  {
    timestamps: true,
  }
);

userSchema.pre('save', async function (next) {
  if (!this.isModified('password')) {
    return next();
  }

  this.password = await PasswordManager.toHash(this.password);
  return next();
});

export const UserModel = model<UserDoc>('User', userSchema);
