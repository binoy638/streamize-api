export type UserRole = "admin" | "user";

export type User = {
  id: string;
  username: string;
  role: UserRole;
  storageQuotaBytes: number;
  createdAt: string;
  updatedAt: string;
};

export type Health = {
  status?: string;
  service?: string;
  database?: string;
  version?: string;
};

export type TorrentStatus = "added" | "downloading" | "paused" | "queued" | "processing" | "done" | "error";
export type TorrentFileStatus = "downloading" | "queued" | "processing" | "done" | "error";
export type JobStatus = "queued" | "running" | "succeeded" | "failed" | "canceled";
export type WatchPartyControlMode = "host_only" | "everyone";
export type WatchPartyStatus = "active" | "ended";

export type Torrent = {
  id: string;
  ownerUserId: string;
  slug: string;
  magnetUri: string;
  infoHash?: string;
  qbittorrentHash?: string;
  name?: string;
  sizeBytes: number;
  status: TorrentStatus;
  progressPercent: number;
  downloadSpeedBytes: number;
  uploadSpeedBytes: number;
  etaSeconds: number;
  peers: number;
  ratio: number;
  retentionPolicy: "keep" | "delete_after_hls";
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
};

export type TorrentFile = {
  id: string;
  torrentId: string;
  ownerUserId: string;
  slug: string;
  name: string;
  ext: string;
  originalPath?: string;
  hlsPath?: string;
  sizeBytes: number;
  status: TorrentFileStatus;
  progressPreview: boolean;
  transcodingPercent: number;
  downloadPercent: number;
  container?: string;
  videoCodec?: string;
  audioCodec?: string;
  durationSeconds?: number;
  processingMode?: string;
  thumbnailSheetPath?: string;
  thumbnailVttPath?: string;
  directPlayable: boolean;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
};

export type Subtitle = {
  id: string;
  torrentFileId: string;
  fileName: string;
  title: string;
  language: string;
  url: string;
  createdAt: string;
};

export type Job = {
  id: string;
  type: string;
  status: JobStatus;
  payloadJson: string;
  dedupeKey?: string;
  attempts: number;
  maxAttempts: number;
  progressPercent: number;
  leaseUntil?: string;
  lockedBy?: string;
  lastError?: string;
  availableAt: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
  torrentId?: string;
  torrentFileId?: string;
  target?: string;
};

export type WatchParty = {
  id: string;
  ownerUserId: string;
  slug: string;
  torrentFileId: string;
  controlMode: WatchPartyControlMode;
  status: WatchPartyStatus;
  currentPositionSeconds: number;
  durationSeconds: number;
  isPlaying: boolean;
  lastEventAt: string;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
};

export type WatchPartyFile = {
  id: string;
  torrentId: string;
  name: string;
  ext: string;
  sizeBytes: number;
  status: TorrentFileStatus;
  progressPreview: boolean;
  durationSeconds?: number;
  container?: string;
  videoCodec?: string;
  audioCodec?: string;
  hlsReady: boolean;
  directPlayable: boolean;
};

export type WatchPartyParticipant = {
  id: string;
  watchPartyId: string;
  userId?: string;
  displayName: string;
  role: "host" | "guest";
  connected: boolean;
  createdAt: string;
  lastSeenAt: string;
};

export type WatchPartySession = {
  participant: WatchPartyParticipant;
  token: string;
};

export type WatchPartyResponse = {
  party: WatchParty;
  file: WatchPartyFile;
  joinUrl: string;
  session?: WatchPartySession;
};

export type CreateUserInput = {
  username: string;
  password: string;
  role: UserRole;
  storageQuotaBytes: number;
};

export type CreateTorrentInput = {
  magnetUri: string;
  name?: string;
};

export type DeleteTorrentOptions = {
  deleteFiles?: boolean;
  deleteGenerated?: boolean;
};

export type CreateWatchPartyInput = {
  torrentFileId: string;
  controlMode: WatchPartyControlMode;
  displayName?: string;
};

type ApiErrorBody = {
  error?: string;
};

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...init.headers,
    },
  });

  if (!response.ok) {
    let message = response.statusText;
    try {
      const body = (await response.json()) as ApiErrorBody;
      message = body.error || message;
    } catch {
      // Keep the status text if the server did not return JSON.
    }
    throw new ApiError(response.status, message);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export async function signIn(username: string, password: string): Promise<User> {
  const body = await apiFetch<{ user: User }>("/api/auth/sign-in", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
  return body.user;
}

export async function signOut(): Promise<void> {
  await apiFetch<void>("/api/auth/sign-out", { method: "POST" });
}

export async function getMe(): Promise<User> {
  const body = await apiFetch<{ user: User }>("/api/auth/me");
  return body.user;
}

export async function listUsers(): Promise<User[]> {
  const body = await apiFetch<{ users: User[] }>("/api/admin/users");
  return body.users;
}

export async function createUser(input: CreateUserInput): Promise<User> {
  const body = await apiFetch<{ user: User }>("/api/admin/users", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return body.user;
}

export async function listTorrents(): Promise<Torrent[]> {
  const body = await apiFetch<{ torrents: Torrent[] }>("/api/torrents");
  return body.torrents;
}

export async function createTorrent(input: CreateTorrentInput): Promise<Torrent> {
  const body = await apiFetch<{ torrent: Torrent }>("/api/torrents", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return body.torrent;
}

export async function deleteTorrent(id: string, options: DeleteTorrentOptions = {}): Promise<void> {
  const params = new URLSearchParams();
  if (options.deleteFiles !== undefined) {
    params.set("deleteFiles", String(options.deleteFiles));
  }
  if (options.deleteGenerated !== undefined) {
    params.set("deleteGenerated", String(options.deleteGenerated));
  }
  const query = params.toString();
  await apiFetch<void>(`/api/torrents/${encodeURIComponent(id)}${query ? `?${query}` : ""}`, {
    method: "DELETE",
  });
}

export async function listTorrentFiles(torrentId: string): Promise<TorrentFile[]> {
  const body = await apiFetch<{ files: TorrentFile[] }>(`/api/torrents/${encodeURIComponent(torrentId)}/files`);
  return body.files;
}

export async function listAllFiles(): Promise<TorrentFile[]> {
  const body = await apiFetch<{ files: TorrentFile[] }>("/api/files");
  return body.files;
}

export async function listSubtitles(fileId: string): Promise<Subtitle[]> {
  const body = await apiFetch<{ subtitles: Subtitle[] }>(`/api/files/${encodeURIComponent(fileId)}/subtitles`);
  return body.subtitles;
}

export async function listJobs(): Promise<Job[]> {
  const body = await apiFetch<{ jobs: Job[] }>("/api/jobs");
  return body.jobs;
}

export async function retryJob(id: string): Promise<Job> {
  const body = await apiFetch<{ job: Job }>(`/api/jobs/${encodeURIComponent(id)}/retry`, {
    method: "POST",
  });
  return body.job;
}

export async function cancelJob(id: string): Promise<Job> {
  const body = await apiFetch<{ job: Job }>(`/api/jobs/${encodeURIComponent(id)}/cancel`, {
    method: "POST",
  });
  return body.job;
}

export async function getHealth(): Promise<Health> {
  return apiFetch<Health>("/api/health");
}

export async function createWatchParty(input: CreateWatchPartyInput): Promise<WatchPartyResponse> {
  return apiFetch<WatchPartyResponse>("/api/watch-parties", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function getWatchParty(slug: string): Promise<WatchPartyResponse> {
  return apiFetch<WatchPartyResponse>(`/api/watch-parties/join/${encodeURIComponent(slug)}`);
}

export async function joinWatchParty(slug: string, displayName: string): Promise<WatchPartyResponse> {
  return apiFetch<WatchPartyResponse>(`/api/watch-parties/join/${encodeURIComponent(slug)}`, {
    method: "POST",
    body: JSON.stringify({ displayName }),
  });
}

export async function endWatchParty(id: string): Promise<WatchParty> {
  const body = await apiFetch<{ party: WatchParty }>(`/api/watch-parties/${encodeURIComponent(id)}/end`, {
    method: "POST",
  });
  return body.party;
}

export async function listWatchPartySubtitles(
  slug: string,
  fileId: string,
  session: WatchPartySession,
): Promise<Subtitle[]> {
  const body = await apiFetch<{ subtitles: Subtitle[] }>(
    `/api/watch-parties/join/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/subtitles?${watchPartyQuery(session)}`,
  );
  return body.subtitles;
}

export function watchPartyWebSocketURL(slug: string, session: WatchPartySession): string {
  return watchPartyWebSocketURLs(slug, session)[0];
}

export function watchPartyWebSocketURLs(slug: string, session: WatchPartySession): string[] {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const path = `/api/watch-parties/join/${encodeURIComponent(slug)}/ws?${watchPartyQuery(session)}`;
  const urls = [`${protocol}//${window.location.host}${path}`];

  if (window.location.protocol === "http:" && ["5173", "5174"].includes(window.location.port)) {
    urls.push(`ws://${window.location.hostname}:8080${path}`);
    if (window.location.hostname !== "localhost" && window.location.hostname !== "127.0.0.1") {
      urls.push(`ws://localhost:8080${path}`);
    }
  }

  return Array.from(new Set(urls));
}

export function watchPartyHLSPlaylistURL(slug: string, fileId: string, session: WatchPartySession): string {
  return `/api/watch-parties/join/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/hls/index.m3u8?${watchPartyQuery(session)}`;
}

export function watchPartyOriginalURL(slug: string, fileId: string, session: WatchPartySession): string {
  return `/api/watch-parties/join/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/original?${watchPartyQuery(session)}`;
}

export function watchPartyPreviewVTTURL(slug: string, fileId: string, session: WatchPartySession): string {
  return `/api/watch-parties/join/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}/preview/thumbnails.vtt?${watchPartyQuery(session)}`;
}

function watchPartyQuery(session: WatchPartySession): string {
  const params = new URLSearchParams({
    participantId: session.participant.id,
    token: session.token,
  });
  return params.toString();
}
