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

export async function deleteTorrent(id: string): Promise<void> {
  await apiFetch<void>(`/api/torrents/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export async function getHealth(): Promise<Health> {
  return apiFetch<Health>("/api/health");
}
