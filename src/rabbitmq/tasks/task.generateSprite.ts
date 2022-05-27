import { Channel, ConsumeMessage } from 'amqplib';
import fs from 'fs-extra';
import SpriteGenerator from 'sprite-vtt-generator';
import { TorrentPath } from '../../@types';
import { ISpriteGenerationMessageContent } from '../../@types/message';
import logger from '../../config/logger';
import { TorrentModel } from '../../models/torrent.schema';
import Utils from '../../utils';

export const generateSprite =
  (channel: Channel) =>
  async (message: ConsumeMessage | null): Promise<void> => {
    if (!message) return;

    const file = Utils.getMessageContent<ISpriteGenerationMessageContent>(message);

    logger.info(`Received new job to generate sprite :${file.name}`);

    const outputDir = `${TorrentPath.DOWNLOAD}/${file.slug}/thumbnails`;

    await fs.ensureDir(outputDir);

    const vttPath = `${outputDir}/${file.slug}.vtt`;

    try {
      const sprite = new SpriteGenerator({
        inputPath: file.inputPath,
        outputDir,
        multiple: true,
        webVTT: {
          required: true,
          path: vttPath,
        },
      });
      await sprite.generate();
      await TorrentModel.updateOne(
        { _id: file.torrentID, 'files.slug': file.slug },
        { $set: { 'files.$.progressPreview': true } }
      );
      channel.ack(message);
    } catch (error) {
      logger.error(`something went wrong while generating sprite: ${file.name} error: ${JSON.stringify(error)}`);
      channel.ack(message);
    }
  };
