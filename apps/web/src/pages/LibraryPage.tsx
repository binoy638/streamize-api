import { type CSSProperties, type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Layers, Play, Plus, Share2, Trash2 } from "lucide-react";

import * as api from "../lib/api";
import { DeleteTorrentModal } from "../components/DeleteTorrentModal";
import { Badge, Button, EmptyState, Field, Input, Modal, Progress, Textarea } from "../components/ui";
import { CreateShareModal } from "../components/CreateShareModal";
import { formatBytes } from "../lib/format";
import { mediaItems } from "../lib/mock-data";

const pollableTorrentStatuses = new Set<api.TorrentStatus>(["added", "queued", "downloading", "processing"]);
const pollableFileStatuses = new Set<api.TorrentFileStatus>(["queued", "downloading", "processing"]);

type LibraryStatus = "ready" | "processing" | "failed";

type LibraryItem = {
  id: string;
  source: "api" | "mock";
  torrentId: string;
  target: string;
  title: string;
  duration: string;
  meta: string[];
  detail: string;
  status: LibraryStatus;
  tags: string[];
  progress: number;
  watchPercent: number;
  resumeFileId?: string;
  watched: boolean;
  sizeBytes: number;
  pollable: boolean;
  posterHue: number;
  playable: boolean;
  fileCount: number;
  mediaType: api.CatalogMediaType | "mock";
  metadataStatus: api.MetadataStatus | "mock";
  thumbnailUrl?: string;
  posterUrl?: string;
  searchText: string;
};

// A single sprite cell is 160x90 (see transcoding.ProcessSprite). The sheet is
// a grid of these cells, so dividing the sheet width by this gives the column
// count needed to crop the first cell as a poster thumbnail.
const SPRITE_CELL_WIDTH = 160;

// Saved playback below this many seconds is treated as "not started"; at/above
// this fraction of the runtime the video counts as fully watched.
const WATCH_RESUME_MIN_SECONDS = 5;
const WATCH_FINISHED_FRACTION = 0.95;

type Notice = {
  tone: "success" | "warn";
  text: string;
};

export function LibraryPage() {
  const [items, setItems] = useState<LibraryItem[]>([]);
  const [torrents, setTorrents] = useState<api.Torrent[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [usingMock, setUsingMock] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [deletingTorrentId, setDeletingTorrentId] = useState("");
  const [pendingDelete, setPendingDelete] = useState<LibraryItem | null>(null);
  const [magnetOpen, setMagnetOpen] = useState(false);
  const [shareTargetId, setShareTargetId] = useState<string | null>(null);

  const loadLibrary = useCallback(async ({ showLoading = true, fallbackToMock = true } = {}) => {
    if (showLoading) {
      setLoading(true);
    }

    try {
      const [records, libraryItems, progress] = await Promise.all([
        api.listTorrents(),
        api.listLibrary(),
        // Watch progress is a nice-to-have; never fail the whole load over it.
        api.listVideoProgress().catch(() => [] as api.VideoProgress[]),
      ]);

      setTorrents(records);
      setItems(libraryItems.map((item) => apiCatalogItemToLibraryItem(item, progress)));
      setUsingMock(false);
      setError("");
    } catch (err) {
      if (fallbackToMock) {
        setTorrents([]);
        setItems(mediaItems.map(mockMediaToLibraryItem));
        setUsingMock(true);
        setError(err instanceof Error ? err.message : "Unable to load library");
      }
    } finally {
      if (showLoading) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    void loadLibrary();
  }, [loadLibrary]);

  useEffect(() => {
    if (usingMock) {
      return;
    }

    const hasActiveTorrent = torrents.some((torrent) => pollableTorrentStatuses.has(torrent.status));
    const hasActiveFile = items.some((item) => item.source === "api" && item.pollable);
    if (!hasActiveTorrent && !hasActiveFile) {
      return;
    }

    const intervalID = window.setInterval(() => {
      void loadLibrary({ showLoading: false, fallbackToMock: false });
    }, 5000);

    return () => window.clearInterval(intervalID);
  }, [items, loadLibrary, torrents, usingMock]);

  // The library only surfaces videos that are ready to play; in-progress and
  // failed files are reachable from the Torrents tab instead.
  const visibleItems = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return items.filter((item) => {
      if (item.status !== "ready") {
        return false;
      }
      return !normalized || item.searchText.includes(normalized);
    });
  }, [items, query]);

  async function addTorrent(input: api.CreateTorrentInput) {
    setQuery("");
    if (usingMock) {
      const local = mockMediaToLibraryItem({
        ...mediaItems[0],
        id: `mock_${Date.now()}`,
        title: input.name?.trim() || "New magnet",
        status: "processing",
        progress: 0,
        tags: ["processing", "recent"],
        meta: ["Pending", "queued"],
      });
      setItems((current) => [local, ...current]);
      setNotice({ tone: "warn", text: "Prototype media added locally. Sign in through the API to persist magnet submissions." });
      return;
    }

    await api.createTorrent(input);
    await loadLibrary({ showLoading: false, fallbackToMock: false });
    setNotice({ tone: "success", text: "Magnet added — track download progress in the Torrents tab." });
  }

  async function deleteLibraryItem(item: LibraryItem, options: Required<api.DeleteTorrentOptions>) {
    if (item.source === "mock") {
      setItems((current) => current.filter((record) => record.id !== item.id));
      setNotice({ tone: "warn", text: "Prototype media removed locally." });
      setPendingDelete(null);
      return;
    }

    if (!item.torrentId) {
      setNotice({ tone: "warn", text: "This file has no associated torrent." });
      setPendingDelete(null);
      return;
    }

    setDeletingTorrentId(item.torrentId);
    try {
      await api.deleteTorrent(item.torrentId, options);
      await loadLibrary({ showLoading: false, fallbackToMock: false });
      setNotice({ tone: "success", text: "Torrent deleted with selected cleanup options." });
      setPendingDelete(null);
    } catch (err) {
      setNotice({ tone: "warn", text: err instanceof Error ? err.message : "Unable to delete media." });
    } finally {
      setDeletingTorrentId("");
    }
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Media Library</span>
          <h1>Everything that's ready to watch.</h1>
          <p>Finished videos with HLS output or direct playback. Downloads still in progress live in the Torrents tab.</p>
        </div>
        <div className="header-actions">
          <Button variant="primary" onClick={() => setMagnetOpen(true)}>
            <Plus size={16} /> Add magnet
          </Button>
        </div>
      </div>

      {error ? <div className="alert alert-warn is-visible">Using prototype media because the API did not respond: {error}</div> : null}
      {notice ? <div className={`alert alert-${notice.tone} is-visible`}>{notice.text}</div> : null}

      <div className="toolbar">
        <span className="toolbar-count mr-auto">
          {visibleItems.length} {visibleItems.length === 1 ? "video" : "videos"}
        </span>
        <label className="inline-search">
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search the library"
            aria-label="Search the library"
          />
        </label>
      </div>

      {visibleItems.length === 0 ? (
        <EmptyState title="Nothing ready to watch yet">
          {loading
            ? "Loading your library."
            : query
              ? "No ready videos match your search."
              : "Videos appear here once they finish processing. Check the Torrents tab for downloads in progress."}
        </EmptyState>
      ) : (
        <div className="media-grid">
          {visibleItems.map((item) => {
            const cardTarget = item.target || (item.torrentId ? `/torrents/${item.torrentId}` : "/library");
            const deleting = !!item.torrentId && deletingTorrentId === item.torrentId;
            return (
              <article className="media-card" key={item.id}>
                <Link
                  className="poster-link"
                  to={cardTarget}
                  aria-label={item.playable ? `Play ${item.title}` : `Open ${item.title}`}
                >
                  <div
                    className={item.playable ? "poster" : "poster poster-static"}
                    style={{ "--poster-hue": item.posterHue } as CSSProperties}
                  >
                    {item.metadataStatus === "unmatched" || item.metadataStatus === "failed" ? (
                      <Badge tone="warn">metadata {item.metadataStatus}</Badge>
                    ) : null}
                    {item.posterUrl ? <img src={item.posterUrl} alt="" aria-hidden className="poster-art" /> : null}
                    {item.thumbnailUrl ? <SpriteThumbnail src={item.thumbnailUrl} /> : null}
                    <span className="duration">{item.duration}</span>
                    {item.posterUrl || item.thumbnailUrl ? null : (
                      <span className="poster-label">{item.title.split(":")[0]}</span>
                    )}
                    {item.watchPercent > 0 ? (
                      <span className="poster-progress" aria-hidden>
                        <span style={{ width: `${item.watchPercent}%` }} />
                      </span>
                    ) : null}
                  </div>
                </Link>
                <div className="media-body">
                  <div className="media-title" title={item.title}>
                    {item.title}
                  </div>
                  <div className="media-meta" title={item.detail}>
                    {item.meta.join(" · ")}
                  </div>
                  <div className="progress-row">
                    <Progress value={item.watchPercent} />
                    <span>{watchLabel(item)}</span>
                  </div>
                  <div className="media-actions">
                    {item.resumeFileId ? (
                      <Link className="btn btn-primary flex-1" to={`/player/${item.resumeFileId}`}>
                        <Play size={15} /> Continue
                      </Link>
                    ) : item.playable ? (
                      <Link className="btn btn-primary flex-1" to={cardTarget}>
                        {item.fileCount > 1 ? <Layers size={15} /> : <Play size={15} />}
                        {item.fileCount > 1 ? "Open" : item.watched ? "Watch again" : "Watch"}
                      </Link>
                    ) : (
                      <Link className="btn flex-1" to={cardTarget}>
                        Details
                      </Link>
                    )}
                    <Button
                      className="w-9 flex-none px-0"
                      title={item.source === "mock" ? "Sign in to share" : "Create share link"}
                      aria-label="Create share link"
                      disabled={item.source === "mock"}
                      onClick={() => setShareTargetId(item.id)}
                    >
                      <Share2 size={15} />
                    </Button>
                    <Button
                      variant="danger"
                      className="w-9 flex-none px-0"
                      disabled={!item.torrentId || deleting}
                      title={!item.torrentId ? "No associated torrent" : deleting ? "Deleting…" : "Delete torrent"}
                      aria-label="Delete torrent"
                      onClick={() => setPendingDelete(item)}
                    >
                      <Trash2 size={15} />
                    </Button>
                  </div>
                </div>
              </article>
            );
          })}
        </div>
      )}

      <AddMagnetModal open={magnetOpen} onClose={() => setMagnetOpen(false)} onCreate={addTorrent} />
      <CreateShareModal
        open={shareTargetId !== null}
        presetItemId={shareTargetId ?? undefined}
        onClose={() => setShareTargetId(null)}
        onCreated={() => setNotice({ tone: "success", text: "Share link created and copied to clipboard." })}
      />
      <DeleteTorrentModal
        open={pendingDelete !== null}
        title={pendingDelete?.title || "this torrent"}
        busy={deletingTorrentId !== ""}
        onClose={() => {
          if (!deletingTorrentId) {
            setPendingDelete(null);
          }
        }}
        onConfirm={(options) => (pendingDelete ? deleteLibraryItem(pendingDelete, options) : undefined)}
      />
    </section>
  );
}

function apiCatalogItemToLibraryItem(item: api.LibraryCatalogItem, watchProgress: api.VideoProgress[]): LibraryItem {
  const files = item.files;
  const primary = files.find((file) => Boolean(file.hlsPath) || file.directPlayable) || files[0];
  const playable = files.some((file) => Boolean(file.hlsPath) || file.directPlayable);
  const failed = files.length > 0 && files.every((file) => file.status === "error");
  const status: LibraryStatus = failed ? "failed" : playable ? "ready" : "processing";
  const progress = playable ? 100 : aggregateProgress(files);
  const transferLabel = primary?.processingMode ? primary.processingMode.replace(/_/g, " ") : (primary?.status ?? item.metadataStatus);
  const sizeBytes = files.reduce((total, file) => total + (file.sizeBytes || 0), 0);
  const fileCount = files.length;
  const episodeCount = files.filter((file) => file.episode).length;
  const typeLabel = item.mediaType === "unknown" ? item.metadataStatus : item.mediaType;
  const meta = [
    sizeBytes > 0 ? formatBytes(sizeBytes) : "Pending",
    fileCount === 1 ? "1 file" : `${fileCount} files`,
    episodeCount > 0 ? `${episodeCount} episodes` : typeLabel,
    transferLabel,
  ];
  // detail is the full string surfaced on hover; it adds the codec, which is
  // too long to keep on the always-visible single-line meta row.
  const detail = [...meta, primary ? codecLabel(primary) : ""].filter(Boolean).join(" · ");
  const tags = [status, item.mediaType, item.metadataStatus, primary?.status].filter(Boolean) as string[];
  const target = fileCount === 1 && playable && primary ? `/player/${primary.id}` : `/library/${encodeURIComponent(item.id)}`;

  // watchProgress arrives most-recently-watched first, so the first match is
  // the file the user touched last within this catalog item.
  const fileIDs = new Set(files.map((file) => file.id));
  const watch = watchProgress.find((record) => fileIDs.has(record.torrentFileId));
  const watchedFile = watch ? files.find((file) => file.id === watch.torrentFileId) : undefined;
  const watchDuration = watch && watch.durationSeconds > 0 ? watch.durationSeconds : watchedFile?.durationSeconds || 0;
  const watchPosition = watch ? Math.max(0, watch.positionSeconds || 0) : 0;
  const watchPercent = watchDuration > 0 ? clampPercent((watchPosition / watchDuration) * 100) : 0;
  const watched = watchDuration > 0 && watchPosition >= watchDuration * WATCH_FINISHED_FRACTION;
  const resumeFileId = watch && !watched && watchPosition >= WATCH_RESUME_MIN_SECONDS ? watch.torrentFileId : undefined;

  return {
    id: item.id,
    source: "api",
    torrentId: primary?.torrentId || "",
    target,
    title: item.title,
    duration: primary ? formatDuration(primary.durationSeconds || 0) : "Pending",
    meta,
    detail,
    status,
    tags,
    progress,
    watchPercent,
    resumeFileId,
    watched,
    sizeBytes,
    pollable: files.some((file) => pollableFileStatuses.has(file.status)),
    posterHue: hueFromID(item.id),
    playable,
    fileCount,
    mediaType: item.mediaType,
    metadataStatus: item.metadataStatus,
    thumbnailUrl: primary ? previewSpriteURL(primary) : undefined,
    posterUrl: item.posterUrl,
    searchText: [item.title, item.originalTitle ?? "", item.mediaType, item.metadataStatus, ...files.map((file) => file.name), ...meta].join(" ").toLowerCase(),
  };
}

function aggregateProgress(files: api.LibraryFile[]): number {
  if (files.length === 0) {
    return 0;
  }
  const total = files.reduce((sum, file) => {
    const progress = file.status === "done" ? 100 : file.transcodingPercent || file.downloadPercent || 0;
    return sum + clampPercent(progress);
  }, 0);
  return clampPercent(total / files.length);
}

// previewSpriteURL builds the URL of the generated thumbnail sprite sheet, or
// returns undefined when the file has no preview asset yet.
function previewSpriteURL(file: api.TorrentFile): string | undefined {
  if (!file.thumbnailSheetPath) {
    return undefined;
  }
  const assetName = file.thumbnailSheetPath.split(/[\\/]/).pop();
  if (!assetName) {
    return undefined;
  }
  return `/api/files/${encodeURIComponent(file.id)}/preview/${encodeURIComponent(assetName)}`;
}

// SpriteThumbnail renders the first cell of a thumbnail sprite sheet as a
// poster image. The sheet is a grid of 160x90 cells; once it loads, the image
// is scaled so the top-left cell exactly fills the (16:9) poster.
function SpriteThumbnail({ src }: { src: string }) {
  const [columns, setColumns] = useState(0);

  return (
    <img
      src={src}
      alt=""
      aria-hidden
      className="poster-thumb"
      style={columns > 0 ? { width: `${columns * 100}%`, opacity: 1 } : undefined}
      onLoad={(event) => {
        const { naturalWidth } = event.currentTarget;
        if (naturalWidth > 0) {
          setColumns(naturalWidth / SPRITE_CELL_WIDTH);
        }
      }}
      onError={(event) => {
        event.currentTarget.style.display = "none";
      }}
    />
  );
}

function mockMediaToLibraryItem(item: (typeof mediaItems)[number]): LibraryItem {
  const status = item.status === "failed" ? "failed" : item.status === "ready" ? "ready" : "processing";
  return {
    id: item.id,
    source: "mock",
    torrentId: item.torrentId,
    target: status === "ready" ? `/player/${item.id}` : `/torrents/${item.torrentId}`,
    title: item.title,
    duration: item.duration,
    meta: item.meta,
    detail: item.meta.join(" · "),
    status,
    tags: item.tags,
    progress: item.progress,
    watchPercent: 0,
    resumeFileId: undefined,
    watched: false,
    sizeBytes: sizeFromMeta(item.meta[0]),
    pollable: status === "processing",
    posterHue: item.posterHue,
    playable: status === "ready",
    fileCount: 1,
    mediaType: "mock",
    metadataStatus: "mock",
    searchText: [item.title, item.status, ...item.meta].join(" ").toLowerCase(),
  };
}

function watchLabel(item: LibraryItem): string {
  if (item.watched) {
    return "Watched";
  }
  if (item.watchPercent > 0) {
    return `${item.watchPercent}% watched`;
  }
  return "Not started";
}

function codecLabel(file: api.TorrentFile): string {
  const codecs = [file.videoCodec, file.audioCodec].filter(Boolean);
  if (codecs.length > 0) {
    return codecs.map((codec) => String(codec).toUpperCase()).join(" / ");
  }

  const extension = file.ext || (file.name.includes(".") ? file.name.slice(file.name.lastIndexOf(".")) : "");
  return extension ? extension.replace(".", "").toUpperCase() : "Pending";
}

function formatDuration(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) {
    return "Pending";
  }

  const seconds = Math.round(totalSeconds);
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (minutes < 60) {
    return `${minutes}:${String(remainingSeconds).padStart(2, "0")}`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return `${hours}:${String(remainingMinutes).padStart(2, "0")}:${String(remainingSeconds).padStart(2, "0")}`;
}

function hueFromID(value: string): number {
  let hash = 0;
  for (const char of value) {
    hash = (hash * 31 + char.charCodeAt(0)) % 360;
  }
  return hash;
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.round(Math.max(0, Math.min(100, value)));
}

function sizeFromMeta(value: string): number {
  const [amountText, unit = "B"] = value.split(" ");
  const amount = Number(amountText);
  if (!Number.isFinite(amount)) {
    return 0;
  }

  switch (unit.toUpperCase()) {
    case "TB":
      return amount * 1024 ** 4;
    case "GB":
      return amount * 1024 ** 3;
    case "MB":
      return amount * 1024 ** 2;
    case "KB":
      return amount * 1024;
    default:
      return amount;
  }
}

function AddMagnetModal({
  open,
  onClose,
  onCreate,
}: {
  open: boolean;
  onClose: () => void;
  onCreate: (input: api.CreateTorrentInput) => Promise<void>;
}) {
  const [magnet, setMagnet] = useState("");
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setMessage("");
    if (!magnet.trim().match(/^magnet:\?xt=/)) {
      setMessage("Paste a valid magnet URL beginning with magnet:?xt=.");
      return;
    }

    setBusy(true);
    try {
      await onCreate({ magnetUri: magnet.trim(), name: name.trim() || undefined });
      setMagnet("");
      setName("");
      onClose();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Unable to add torrent.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title="Add Magnet"
      open={open}
      onClose={onClose}
      footer={
        <>
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" form="add-library-magnet-form" disabled={busy}>
            {busy ? "Adding..." : "Add to queue"}
          </Button>
        </>
      }
    >
      <form id="add-library-magnet-form" className="contents" onSubmit={submit}>
        {message ? <div className="alert alert-error is-visible">{message}</div> : null}
        <Field label="Magnet URL">
          <Textarea
            value={magnet}
            onChange={(event) => setMagnet(event.target.value)}
            placeholder="magnet:?xt=urn:btih:..."
          />
        </Field>
        <Field label="Display name">
          <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Optional" />
        </Field>
      </form>
    </Modal>
  );
}
