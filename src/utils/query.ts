/* eslint-disable unicorn/no-null */
import { Document } from 'mongoose';
import { ITorrent, IVideo } from '../@types';
import { TorrentModel } from '../models/torrent.schema';
import { allowedExt } from './misc';

export const createTorrentWithMagnet = async (magnet: string): Promise<Document> => {
  const doc = new TorrentModel({ magnet, status: 'added' });
  return doc.save();
};

export const updateNoMediaTorrent = async (_id: string): Promise<void> => {
  await TorrentModel.updateOne({ _id }, { noMedia: true, status: 'error' });
};

export const addVideoFiles = async (_id: string, videoFile: IVideo): Promise<void> => {
  try {
    await TorrentModel.updateOne({ _id }, { $push: { files: { ...videoFile } } });
  } catch (error) {
    console.log(error);
  }
};

export const getVideoFiles = async (_id: string): Promise<IVideo[] | null> => {
  const doc = await TorrentModel.findOne({ _id });
  console.log(doc);
  if (!doc) return null;
  console.log(doc);
  return doc.files.filter(file => allowedExt.has(file.ext));
};

export const clearTorrents = async (): Promise<void> => {
  await TorrentModel.deleteMany({});
};

export const updateTorrentInfo = async (_id: string, data: Partial<ITorrent>): Promise<ITorrent> => {
  const doc = await TorrentModel.findOneAndUpdate({ _id }, data, { lean: true, new: true });
  if (!doc) throw new Error("Counldn't find torrent");
  return doc;
};

export const updateTorrentFileStatus = async (_id: string, slug: string, status: string): Promise<void> => {
  await TorrentModel.updateOne({ _id, 'files.slug': slug }, { $set: { 'files.$.status': status } });
};

export const updateFileConvertProgress = async (
  _id: string,
  slug: string,
  progress: number,
  state: string
): Promise<void> => {
  await TorrentModel.updateOne(
    { _id, 'files.slug': slug },
    { $set: { 'files.$.progress': progress, 'files.$.state': state } }
  );
};

export const doesTorrentAlreadyExist = async (magnet: string): Promise<boolean> => {
  const doc = await TorrentModel.findOne({ magnet });
  if (!doc) return false;
  return true;
};
