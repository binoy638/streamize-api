/* eslint-disable unicorn/no-null */
import { Document } from 'mongoose';
import { ConvertState, IDownloadInfo, ITorrent, IVideo, TorrentStatus } from '../@types';
import logger from '../config/logger';
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

export const updateTorrentFileStatus = async (_id: string, slug: string, status: TorrentStatus): Promise<void> => {
  await TorrentModel.updateOne({ _id, 'files.slug': slug }, { $set: { 'files.$.status': status } });
};

export const updateFileConvertProgress = async (
  _id: string,
  slug: string,
  progress: number,
  state: ConvertState
): Promise<void> => {
  try {
    await TorrentModel.updateOne(
      { _id, 'files.slug': slug },
      { $set: { 'files.$.convertStatus': { progress, state } } }
    );
  } catch (error) {
    logger.error(error);
  }
};

export const doesTorrentAlreadyExist = async (magnet: string): Promise<boolean> => {
  const doc = await TorrentModel.findOne({ magnet });
  if (!doc) return false;
  return true;
};

export const updateTorrentDownloadInfo = async (_id: string, downloadInfo: IDownloadInfo): Promise<void> => {
  try {
    await TorrentModel.findByIdAndUpdate(_id, { $set: { downloadInfo } });
  } catch (error) {
    logger.error(error);
  }
};

export const updateFilePath = async (_id: string, slug: string, path: string): Promise<void> => {
  try {
    await TorrentModel.updateOne({ _id, 'files.slug': slug }, { $set: { 'files.$.path': path } });
  } catch (error) {
    logger.error(error);
  }
};

export const getTorrentBySlug = async (slug: string): Promise<ITorrent | null> => {
  try {
    const doc = await TorrentModel.findOne({ slug });
    if (!doc) return null;
    return doc;
  } catch (error) {
    logger.error(error);
    throw new Error('something went wrong while fetching torrent by slug');
  }
};

export const getTorrentByMagnet = async (magnet: string): Promise<ITorrent | null> => {
  try {
    const doc = await TorrentModel.findOne({ magnet });
    if (!doc) return null;
    return doc;
  } catch (error) {
    logger.error(error);
    throw new Error('something went wrong while fetching torrent by magnet');
  }
};

export const deleteTorrentByID = async (_id: string): Promise<void> => {
  try {
    await TorrentModel.deleteOne({ _id });
  } catch (error) {
    logger.error(error);
  }
};

//! need pagination later
export const getAllTorrentsFromDB = async (): Promise<ITorrent[]> => {
  try {
    const docs = await TorrentModel.find({}).limit(20);
    if (!docs) return [];
    return docs;
  } catch (error) {
    logger.error(error);
    throw new Error('something went wrong while fetching all torrents');
  }
};

//! check if torrent is completly downloaded
export const getVideoFile = async (torrentSlug: string, videoSlug: string): Promise<IVideo | null | undefined> => {
  try {
    const doc = await TorrentModel.findOne({ slug: torrentSlug, 'files.slug': videoSlug });
    if (!doc) return null;
    return doc.files.find(file => file.slug === videoSlug);
  } catch (error) {
    logger.error(error);
    throw new Error('something went wrong while fetching video file');
  }
};
