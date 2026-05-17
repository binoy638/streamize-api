import type { User } from "./api";

export type Status =
  | "ready"
  | "processing"
  | "failed"
  | "downloading"
  | "paused"
  | "queued"
  | "running"
  | "succeeded"
  | "canceled"
  | "online"
  | "offline";

export type MediaItem = {
  id: string;
  torrentId: string;
  title: string;
  duration: string;
  meta: string[];
  size: string;
  progress: number;
  status: "ready" | "processing" | "failed";
  tags: string[];
  posterHue: number;
};

export type Torrent = {
  id: string;
  name: string;
  size: string;
  progress: number;
  status: "downloading" | "paused" | "queued" | "processing" | "done" | "error";
  speed: string;
  eta: string;
  peers: number;
  ratio: number;
  addedAt: string;
};

export type TorrentFile = {
  id: string;
  torrentId: string;
  name: string;
  size: string;
  status: "ready" | "processing" | "failed";
  progress: number;
  codec: string;
  subtitles: number;
};

export type Job = {
  id: string;
  type: string;
  target: string;
  status: "queued" | "running" | "succeeded" | "failed" | "canceled";
  progress: number;
  attempts: string;
  worker: string;
  updatedAt: string;
  error?: string;
};

export type Share = {
  id: string;
  title: string;
  scope: "Single video" | "Full torrent";
  url: string;
  expiresAt: string;
  status: "online" | "paused" | "offline";
  createdBy: string;
};

export const demoUser: User = {
  id: "usr_demo_admin",
  username: "admin",
  role: "admin",
  storageQuotaBytes: 0,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-05-17T00:00:00Z",
};

export const mockUsers: User[] = [
  demoUser,
  {
    id: "usr_mira",
    username: "mira",
    role: "admin",
    storageQuotaBytes: 858993459200,
    createdAt: "2026-01-14T08:15:00Z",
    updatedAt: "2026-05-12T09:20:00Z",
  },
  {
    id: "usr_kai",
    username: "kai",
    role: "user",
    storageQuotaBytes: 161061273600,
    createdAt: "2026-03-21T12:45:00Z",
    updatedAt: "2026-05-13T18:32:00Z",
  },
  {
    id: "usr_guest",
    username: "guest-review",
    role: "user",
    storageQuotaBytes: 26843545600,
    createdAt: "2026-05-08T10:00:00Z",
    updatedAt: "2026-05-08T10:00:00Z",
  },
];

export const mediaItems: MediaItem[] = [
  {
    id: "file_neon_harbor",
    torrentId: "tor_neon_harbor",
    title: "Neon Harbor S02E04",
    duration: "02:18:44",
    meta: ["1080p HLS", "2 audio", "6.2 GB"],
    size: "6.2 GB",
    progress: 100,
    status: "ready",
    tags: ["ready", "recent"],
    posterHue: 235,
  },
  {
    id: "file_signal_ridge",
    torrentId: "tor_signal_ridge",
    title: "Signal Ridge",
    duration: "01:44:08",
    meta: ["ffprobe done", "HLS queue", "8.1 GB"],
    size: "8.1 GB",
    progress: 62,
    status: "processing",
    tags: ["processing", "recent"],
    posterHue: 195,
  },
  {
    id: "file_archive_7",
    torrentId: "tor_archive_7",
    title: "Archive 7: Pilot",
    duration: "00:48:13",
    meta: ["720p HLS", "Subs", "1.9 GB"],
    size: "1.9 GB",
    progress: 100,
    status: "ready",
    tags: ["ready"],
    posterHue: 280,
  },
  {
    id: "file_north_cache",
    torrentId: "tor_north_cache",
    title: "North Cache",
    duration: "--:--",
    meta: ["bad MKV index", "4.3 GB"],
    size: "4.3 GB",
    progress: 38,
    status: "failed",
    tags: ["failed"],
    posterHue: 25,
  },
  {
    id: "file_field_notes",
    torrentId: "tor_field_notes",
    title: "Field Notes 2026",
    duration: "02:02:19",
    meta: ["4K source", "1080p output", "13.8 GB"],
    size: "13.8 GB",
    progress: 100,
    status: "ready",
    tags: ["ready"],
    posterHue: 155,
  },
  {
    id: "file_glass_city",
    torrentId: "tor_glass_city",
    title: "Glass City",
    duration: "01:32:50",
    meta: ["thumbnail pass", "3.4 GB"],
    size: "3.4 GB",
    progress: 81,
    status: "processing",
    tags: ["processing"],
    posterHue: 320,
  },
];

export const torrents: Torrent[] = [
  {
    id: "tor_neon_harbor",
    name: "Neon Harbor S02 1080p",
    size: "42.6 GB",
    progress: 100,
    status: "done",
    speed: "0 B/s",
    eta: "Complete",
    peers: 18,
    ratio: 1.8,
    addedAt: "2026-05-14",
  },
  {
    id: "tor_signal_ridge",
    name: "Signal Ridge 2026 WEB-DL",
    size: "8.1 GB",
    progress: 62,
    status: "processing",
    speed: "0 B/s",
    eta: "HLS queue",
    peers: 9,
    ratio: 0.7,
    addedAt: "2026-05-16",
  },
  {
    id: "tor_archive_7",
    name: "Archive 7 Season 1",
    size: "18.9 GB",
    progress: 100,
    status: "done",
    speed: "0 B/s",
    eta: "Complete",
    peers: 24,
    ratio: 2.3,
    addedAt: "2026-05-10",
  },
  {
    id: "tor_north_cache",
    name: "North Cache Remux",
    size: "4.3 GB",
    progress: 38,
    status: "error",
    speed: "0 B/s",
    eta: "Needs retry",
    peers: 3,
    ratio: 0.2,
    addedAt: "2026-05-12",
  },
  {
    id: "tor_field_notes",
    name: "Field Notes 2026 4K",
    size: "13.8 GB",
    progress: 100,
    status: "done",
    speed: "0 B/s",
    eta: "Complete",
    peers: 11,
    ratio: 1.1,
    addedAt: "2026-05-03",
  },
  {
    id: "tor_glass_city",
    name: "Glass City 2026",
    size: "3.4 GB",
    progress: 81,
    status: "processing",
    speed: "0 B/s",
    eta: "Thumbnail pass",
    peers: 7,
    ratio: 0.9,
    addedAt: "2026-05-15",
  },
];

export const torrentFiles: TorrentFile[] = [
  {
    id: "file_neon_harbor",
    torrentId: "tor_neon_harbor",
    name: "Neon.Harbor.S02E04.1080p.mkv",
    size: "6.2 GB",
    status: "ready",
    progress: 100,
    codec: "H.264 / AAC",
    subtitles: 4,
  },
  {
    id: "file_neon_extra",
    torrentId: "tor_neon_harbor",
    name: "Behind.The.Signal.mkv",
    size: "1.1 GB",
    status: "processing",
    progress: 54,
    codec: "H.265 / AC3",
    subtitles: 1,
  },
  {
    id: "file_signal_ridge",
    torrentId: "tor_signal_ridge",
    name: "Signal.Ridge.2026.WEB-DL.mkv",
    size: "8.1 GB",
    status: "processing",
    progress: 62,
    codec: "H.265 / AAC",
    subtitles: 2,
  },
  {
    id: "file_north_cache",
    torrentId: "tor_north_cache",
    name: "North.Cache.Remux.mkv",
    size: "4.3 GB",
    status: "failed",
    progress: 38,
    codec: "MPEG-4 / DTS",
    subtitles: 0,
  },
];

export const jobs: Job[] = [
  {
    id: "job_probe_signal",
    type: "ffprobe",
    target: "Signal Ridge",
    status: "succeeded",
    progress: 100,
    attempts: "1/3",
    worker: "worker-01",
    updatedAt: "2 min ago",
  },
  {
    id: "job_hls_signal",
    type: "hls-generate",
    target: "Signal Ridge",
    status: "running",
    progress: 62,
    attempts: "1/3",
    worker: "worker-02",
    updatedAt: "now",
  },
  {
    id: "job_thumb_glass",
    type: "thumbnail-sprite",
    target: "Glass City",
    status: "running",
    progress: 81,
    attempts: "1/3",
    worker: "worker-01",
    updatedAt: "45 sec ago",
  },
  {
    id: "job_probe_north",
    type: "ffprobe",
    target: "North Cache",
    status: "failed",
    progress: 38,
    attempts: "3/3",
    worker: "worker-03",
    updatedAt: "18 min ago",
    error: "bad MKV index",
  },
  {
    id: "job_sub_archive",
    type: "subtitle-extract",
    target: "Archive 7: Pilot",
    status: "queued",
    progress: 0,
    attempts: "0/3",
    worker: "unclaimed",
    updatedAt: "queued",
  },
];

export const shares: Share[] = [
  {
    id: "shr_neon",
    title: "Neon Harbor S02E04",
    scope: "Single video",
    url: "https://streamize.local/s/nh-24h",
    expiresAt: "2026-05-18 21:30",
    status: "online",
    createdBy: "admin",
  },
  {
    id: "shr_archive",
    title: "Archive 7 Season 1",
    scope: "Full torrent",
    url: "https://streamize.local/s/archive-week",
    expiresAt: "2026-05-24 09:00",
    status: "paused",
    createdBy: "mira",
  },
  {
    id: "shr_guest",
    title: "Guest review cut",
    scope: "Single video",
    url: "https://streamize.local/s/review-expired",
    expiresAt: "2026-05-10 12:00",
    status: "offline",
    createdBy: "admin",
  },
];

export const settings = {
  mediaRoot: "/mnt/media",
  databasePath: "/var/lib/streamize/streamize.db",
  qbittorrent: "Connected",
  retention: "Keep originals after HLS",
  workers: 3,
  storageUsed: "7.8 TB",
  storagePercent: 68,
};

export function findMedia(id?: string): MediaItem {
  return mediaItems.find((item) => item.id === id) || mediaItems[0];
}

export function findTorrent(id?: string): Torrent {
  return torrents.find((torrent) => torrent.id === id) || torrents[0];
}

export function filesForTorrent(torrentId: string): TorrentFile[] {
  return torrentFiles.filter((file) => file.torrentId === torrentId);
}
