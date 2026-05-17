import ffmpeg from 'fluent-ffmpeg';
import fs from 'fs-extra';
import { ISubtitle, IVideo, TorrentPath } from '../@types';
import logger from '../config/logger';
import { TorrentModel } from '../models/torrent.schema';

interface Sub {
  title: string;
  language: string;
}

class SubtitleProcessor {
  public baseToDir = TorrentPath.SUBTITLES;

  public readonly id: string;

  public readonly video: IVideo;

  constructor(id: string, video: IVideo) {
    this.id = id;
    this.video = video;
  }

  public async findAvailableSubs(): Promise<Sub[]> {
    return new Promise<Sub[]>((resolve, reject) => {
      ffmpeg(this.video.path).ffprobe((err, data) => {
        if (err) {
          logger.error(err);
          reject(err);
        } else {
          const subtitlesCodecs = data.streams
            .filter(stream => stream.codec_type === 'subtitle')
            .map(subtitle => {
              return {
                title: subtitle?.tags?.title || 'unknown',
                language: subtitle?.tags?.language || subtitle?.tags?.LANGUAGE || 'unknown',
              };
            });
          resolve(subtitlesCodecs);
        }
      });
    });
  }

  public async extractSubs(): Promise<void> {
    const subtitleslist = await this.findAvailableSubs().catch(error => {
      logger.error(error);
    });

    if (!subtitleslist || subtitleslist.length === 0) {
      logger.info('no subtitles found');
      return;
    }

    logger.info('Extracting subtitles');
    try {
      const subtitleDir = `${this.baseToDir}/${this.video.slug}`;
      await fs.ensureDir(subtitleDir);
      const promises = subtitleslist.map((subtitle, index) => {
        const fileName = `${this.video.slug}-${subtitle.language}-${index}.vtt`;
        const subtitleInfo: ISubtitle = {
          fileName,
          title: subtitle.title,
          language: subtitle.language,
          path: `${subtitleDir}/${fileName}`,
        };
        return new Promise<void>((resolve, reject) => {
          ffmpeg(this.video.path)
            .noAudio()
            .noVideo()
            .outputOptions('-map', `0:s:${index}`)
            .output(subtitleInfo.path)
            .on('end', () => {
              logger.info(`Subtitle extracted successfully ${subtitleInfo}`);
              this.updateDB(subtitleInfo)
                .then(() => resolve())
                .catch(error => {
                  logger.error(error);
                  reject(error);
                });
            })
            .on('error', error => {
              logger.error(`Subtitle extraction failed ${subtitleInfo}`);
              logger.error(error.message);
              reject();
            })
            .run();
        });
      });

      await Promise.allSettled(promises).catch(error => {
        logger.error(error);
      });
    } catch (error) {
      logger.error(error);
    }
  }

  private async updateDB(subtitle: ISubtitle): Promise<void> {
    try {
      await TorrentModel.updateOne(
        { _id: this.id, 'files.slug': this.video.slug },
        { $push: { 'files.$.subtitles': subtitle } }
      );
    } catch (error) {
      logger.error(error);
    }
  }
}

export default SubtitleProcessor;
