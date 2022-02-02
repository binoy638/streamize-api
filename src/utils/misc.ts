import { ConsumeMessage } from 'amqplib';
import path from 'path';
import { TorrentPath } from '../@types';

export const allowedExt = new Set(['mp4', 'mkv', 'avi']);
export const convertableExt = new Set(['mkv', 'avi']);

export function getMessageContent<T>(message: ConsumeMessage): T {
  if (typeof message.content === 'string') {
    return message.content as T;
  }
  return JSON.parse(message.content.toString()) as T;
}

export const stripFileExtension = (fileName: string): string => {
  return path.parse(fileName).name;
};

export const getFileOutputPath = (fileName: string, path: TorrentPath): string => {
  return `${path}/${stripFileExtension(fileName)}.mp4`;
};
