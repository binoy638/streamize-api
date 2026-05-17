import WebTorrent from 'webtorrent';
import logger from './logger';

const client = new WebTorrent();

client.on('torrent', torrent => {
  logger.info(`Torrent is ready, InfoHash: ${torrent.infoHash}`);
});

client.on('error', err => {
  logger.error(err);
});

export default client;
