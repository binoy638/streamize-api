/* eslint-disable sonarjs/cognitive-complexity */
import ffmpeg from 'fluent-ffmpeg';
import fs from 'fs-extra';
import { IVideo, TorrentPath, TorrentState, VideoState } from '../@types';
import logger from '../config/logger';
import { config } from '../config/stream';
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
      // libmp3lame;
      let audioCodec = 'aac';
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

  // TODO: check video bitrate and adjust hls conversion settings
  // static async getVideoBitrate(path: string): Promise<number> {
  //   return new Promise<number>((resolve, reject) => {
  //     ffmpeg(path).ffprobe((error, data) => {
  //       if (!error) {
  //         if (data.format.bit_rate) {
  //           resolve(data.format.bit_rate / 1000);
  //         } else {
  //           resolve(0);
  //         }
  //       } else {
  //         reject(error);
  //       }
  //     });
  //   });
  // }

  public async convertToHLS(): Promise<string> {
    const { audioCodec } = await this.getCompatibleCodecs();
    const outputPath = `${this.baseDir}/${this.video.slug}/${this.video.slug}.m3u8`;
    await fs.ensureDir(`${this.baseDir}/${this.video.slug}`);
    return new Promise<string>((resolve, reject) => {
      ffmpeg(this.video.path)
        .audioCodec(audioCodec)
        .audioChannels(2)
        // .videoCodec(videoCodec)
        .size('854x480')
        .outputOption(['-sn', `-hls_time ${config.hls_time}`, '-hls_list_size 0', '-f hls'])
        .output(outputPath)
        .on('start', async () => {
          await this.updateTorrentStatus(TorrentState.PROCESSING);
          await this.updateVideoStatus(VideoState.PROCESSING);
          logger.info('Converting to HLS...');
        })
        .on('end', async () => {
          logger.info('conversion done');
          await this.updateTorrentStatus(TorrentState.DONE);
          await this.updateVideoStatus(VideoState.DONE);
          await this.updateTranscodingProgress(100);
          resolve(outputPath);
        })
        .on('error', async err => {
          await this.updateTorrentStatus(TorrentState.ERROR);
          await this.updateVideoStatus(VideoState.ERROR);
          await this.updateTranscodingProgress(0);
          logger.error(err.message);
          reject();
        })
        .on('progress', async progress => {
          logger.debug(`hls video convert progress: ${progress.percent}`);
          await this.updateTranscodingProgress(progress.percent);
        })
        .run();
    });
  }

  private async updateTorrentStatus(status: TorrentState): Promise<void> {
    try {
      await TorrentModel.updateOne({ _id: this.id }, { status });
    } catch (error) {
      logger.error(error);
    }
  }

  private async updateVideoStatus(status: VideoState): Promise<void> {
    try {
      await TorrentModel.updateOne(
        { _id: this.id, 'files.slug': this.video.slug },
        { $set: { 'files.$.status': status } }
      );
    } catch (error) {
      logger.error(error);
    }
  }

  private async updateTranscodingProgress(num: number): Promise<void> {
    try {
      await TorrentModel.updateOne(
        { _id: this.id, 'files.slug': this.video.slug },
        { $set: { 'files.$.transcodingPercent': num } }
      );
    } catch (error) {
      logger.error(error);
    }
  }
}

export default VideoProcessor;
