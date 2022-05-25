/* eslint-disable sonarjs/cognitive-complexity */
import ffmpeg from 'fluent-ffmpeg';
import fs from 'fs-extra';
import { ConvertState, ITorrent, IVideo, TorrentPath, TorrentStatus } from '../@types';
import logger from '../config/logger';
import { TorrentModel } from '../models/torrent.schema';
import SubtitleProcessor from './subtitleProcessor';

class VideoProcessor extends SubtitleProcessor {
  public readonly baseDir = TorrentPath.DOWNLOAD;

  public readonly id: string;

  public readonly video: IVideo;

  public readonly supportedAudioCodecs = new Set(['aac', 'flac', 'mp3', 'ogg', 'opus', 'mpeg', 'mpeg-1', 'mpeg-2']);

  public readonly supportedVideoCodecs = new Set(['avc', 'h264', 'theora', 'vp8', 'vp9']);

  constructor(id: string, video: IVideo) {
    super(id, video);
    this.id = id;
    this.video = video;
  }

  public async getCompatibleCodecs(): Promise<{ audioCodec: string; videoCodec: string }> {
    return new Promise<{ audioCodec: string; videoCodec: string }>((resolve, reject) => {
      let audioCodec = 'libmp3lame';
      let videoCodec = 'libx264';
      ffmpeg(this.video.path).ffprobe((error, data) => {
        if (!error) {
          const audioStreams = data.streams.filter(stream => stream.codec_type === 'audio');
          const videoStreams = data.streams.filter(stream => stream.codec_type === 'video');
          if (audioStreams.length > 0) {
            const audioStream = audioStreams[0];
            if (audioStream?.codec_name && this.supportedAudioCodecs.has(audioStream.codec_name)) {
              logger.info(`Audio codec ${audioStream.codec_name} is supported`);
              audioCodec = 'copy';
            } else {
              logger.info(`Audio codec ${audioStream?.codec_name} is not supported`);
            }
          }
          if (videoStreams.length > 0) {
            const videoStream = videoStreams[0];
            if (videoStream?.codec_name && this.supportedVideoCodecs.has(videoStream.codec_name)) {
              logger.info(`Video codec ${videoStream.codec_name} is supported`);
              videoCodec = 'copy';
            } else {
              logger.info(`Video codec ${videoStream?.codec_name} is not supported`);
            }
          }
          resolve({ audioCodec, videoCodec });
        } else {
          reject(error);
        }
      });
    });
  }

  public async convertToHLS(): Promise<void> {
    const { audioCodec, videoCodec } = await this.getCompatibleCodecs();
    const outputPath = `${this.baseDir}/${this.video.slug}/${this.video.slug}.m3u8`;
    await fs.ensureDir(`${this.baseDir}/${this.video.slug}`);
    return new Promise<void>((resolve, reject) => {
      ffmpeg(this.video.path)
        .audioCodec(audioCodec)
        .videoCodec(videoCodec)
        .outputOption(['-sn', '-hls_time 10', '-hls_list_size 0', '-f hls'])
        .output(outputPath)
        .on('start', async () => {
          await this.updateTorrentInfo({ status: 'converting' });
          await this.updateVideoStatus('converting');
          logger.info('Converting to HLS...');
        })
        .on('end', async () => {
          logger.info('conversion done');
          await this.updateTorrentInfo({ status: 'done' });
          await this.updateVideoStatus('done');
          await this.updateVideoConvertProgress({ progress: 100, state: 'done' });
          resolve();
        })
        .on('error', async err => {
          await this.updateTorrentInfo({ status: 'error' });
          await this.updateVideoStatus('error');
          await this.updateVideoConvertProgress({ progress: 0, state: 'error' });
          logger.error(err.message);
          reject();
        })
        .on('progress', async progress => {
          logger.debug(`hls video convert progress: ${progress.percent}`);
          await this.updateVideoConvertProgress({ progress: progress.percent, state: 'processing' });
        })
        .run();
    });
  }

  private async updateTorrentInfo(data: Partial<ITorrent>): Promise<void> {
    try {
      await TorrentModel.updateOne({ _id: this.id }, data);
    } catch (error) {
      logger.error(error);
    }
  }

  private async updateVideoStatus(status: TorrentStatus): Promise<void> {
    try {
      await TorrentModel.updateOne(
        { _id: this.id, 'files.slug': this.video.slug },
        { $set: { 'files.$.status': status } }
      );
    } catch (error) {
      logger.error(error);
    }
  }

  private async updateVideoConvertProgress(data: { progress: number; state: ConvertState }): Promise<void> {
    try {
      await TorrentModel.updateOne(
        { _id: this.id, 'files.slug': this.video.slug },
        { $set: { 'files.$.convertStatus': data } }
      );
    } catch (error) {
      logger.error(error);
    }
  }
}

export default VideoProcessor;
