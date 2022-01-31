import WebTorrent from 'webtorrent';

const client = new WebTorrent();

client.on('torrent', torrent => {
  console.log({ message: 'Torrent is ready', InfoHash: torrent.infoHash });
});

client.on('error', err => {
  console.log({ err });
});

// const graceful = () => {
//   client.destroy();
// };

// process.on('SIGTERM', graceful);
// process.on('SIGINT', graceful);

export default client;
