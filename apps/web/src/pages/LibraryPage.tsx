import { type CSSProperties, type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import * as api from "../lib/api";
import { DeleteTorrentModal } from "../components/DeleteTorrentModal";
import { Badge, Button, EmptyState, Field, Input, Modal, Progress, Select, StatCard, Textarea } from "../components/ui";
import { formatBytes } from "../lib/format";
import { mediaItems } from "../lib/mock-data";

const filters = [
  { label: "All", value: "all" },
  { label: "Ready", value: "ready" },
  { label: "Processing", value: "processing" },
  { label: "Failed", value: "failed" },
  { label: "Recently added", value: "recent" },
];

const pollableTorrentStatuses = new Set<api.TorrentStatus>(["added", "queued", "downloading", "processing"]);

type LibraryStatus = "ready" | "processing" | "failed";

type LibraryItem = {
  id: string;
  source: "api" | "mock";
  torrentId: string;
  title: string;
  duration: string;
  meta: string[];
  status: LibraryStatus;
  tags: string[];
  progress: number;
  posterHue: number;
  playable: boolean;
  searchText: string;
};

type Notice = {
  tone: "success" | "warn";
  text: string;
};

export function LibraryPage() {
  const [items, setItems] = useState<LibraryItem[]>([]);
  const [torrents, setTorrents] = useState<api.Torrent[]>([]);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [usingMock, setUsingMock] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [deletingTorrentId, setDeletingTorrentId] = useState("");
  const [pendingDelete, setPendingDelete] = useState<LibraryItem | null>(null);
  const [magnetOpen, setMagnetOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);

  const loadLibrary = useCallback(async ({ showLoading = true, fallbackToMock = true } = {}) => {
    if (showLoading) {
      setLoading(true);
    }

    try {
      const records = await api.listTorrents();
      const fileGroups = await Promise.all(
        records.map(async (torrent) => ({
          torrent,
          files: await api.listTorrentFiles(torrent.id),
        })),
      );

      setTorrents(records);
      setItems(fileGroups.flatMap(({ torrent, files }) => files.map((file) => apiFileToLibraryItem(file, torrent))));
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
    const hasActiveFile = items.some((item) => item.source === "api" && item.status === "processing");
    if (!hasActiveTorrent && !hasActiveFile) {
      return;
    }

    const intervalID = window.setInterval(() => {
      void loadLibrary({ showLoading: false, fallbackToMock: false });
    }, 5000);

    return () => window.clearInterval(intervalID);
  }, [items, loadLibrary, torrents, usingMock]);

  const visibleItems = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return items.filter((item) => {
      const matchesFilter = filter === "all" || item.tags.includes(filter);
      const matchesQuery = !normalized || item.searchText.includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, items, query]);

  const stats = useMemo(() => {
    const ready = items.filter((item) => item.status === "ready").length;
    const processing = items.filter((item) => item.status === "processing").length;
    const failed = items.filter((item) => item.status === "failed").length;
    const storage = items
      .filter((item) => item.source === "api")
      .reduce((total, item) => total + sizeFromMeta(item.meta[0]), 0);

    return {
      ready,
      processing,
      failed,
      storage: storage > 0 ? formatBytes(storage) : usingMock ? "7.8 TB" : "0 B",
    };
  }, [items, usingMock]);

  async function addTorrent(input: api.CreateTorrentInput) {
    setFilter("all");
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
    setNotice({ tone: "success", text: "Magnet added and library refresh started." });
  }

  async function deleteLibraryItem(item: LibraryItem, options: Required<api.DeleteTorrentOptions>) {
    if (item.source === "mock") {
      setItems((current) => current.filter((record) => record.id !== item.id));
      setNotice({ tone: "warn", text: "Prototype media removed locally." });
      setPendingDelete(null);
      return;
    }

    setDeletingTorrentId(item.torrentId);
    try {
      await api.deleteTorrent(item.torrentId, options);
      setItems((current) => current.filter((record) => record.torrentId !== item.torrentId));
      setTorrents((current) => current.filter((torrent) => torrent.id !== item.torrentId));
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
          <h1>Ready to watch, still processing, or failed at a glance.</h1>
          <p>Library cards are backed by torrent file records and expose playback readiness plus cleanup actions.</p>
        </div>
        <div className="header-actions">
          <Button variant="primary" onClick={() => setMagnetOpen(true)}>
            Add Magnet
          </Button>
          <Select aria-label="View mode" defaultValue="grid">
            <option value="grid">Grid view</option>
            <option value="list">Compact list</option>
          </Select>
        </div>
      </div>

      {error ? <div className="alert alert-warn is-visible">Using prototype media because the API did not respond: {error}</div> : null}
      {notice ? <div className={`alert alert-${notice.tone} is-visible`}>{notice.text}</div> : null}

      <div className="stats-row">
        <StatCard label="Ready videos" value={String(stats.ready)} detail={usingMock ? "prototype library" : "playable now"} />
        <StatCard label="Processing" value={String(stats.processing)} detail="HLS and thumbnails" />
        <StatCard label="Failed" value={String(stats.failed)} detail="needs retry" />
        <StatCard label="Storage" value={stats.storage} detail={usingMock ? "sample data" : "original media listed"} />
      </div>

      <div className="filters">
        {filters.map((item) => (
          <button
            key={item.value}
            className={`filter-chip ${filter === item.value ? "active" : ""}`}
            onClick={() => setFilter(item.value)}
          >
            {item.label}
          </button>
        ))}
      </div>

      <label className="inline-search">
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Filter visible media"
          aria-label="Filter visible media"
        />
      </label>

      <div className="layout-grid">
        <div className="media-grid">
          {visibleItems.map((item) => (
            <article className="media-card" key={item.id}>
              <div className="poster" style={{ "--poster-hue": item.posterHue } as CSSProperties}>
                <span className="duration">{item.duration}</span>
                <span className="poster-label">{item.title.split(":")[0]}</span>
              </div>
              <div className="media-body">
                <div className="media-title">{item.title}</div>
                <div className="media-meta">
                  {item.meta.map((meta) => (
                    <span key={meta}>{meta}</span>
                  ))}
                </div>
                <div className="progress-row">
                  <Progress value={item.progress} />
                  <span>{item.progress === 100 ? "Ready" : `${item.progress}%`}</span>
                </div>
                <div className="component-row">
                  <Badge tone={item.status}>{statusLabel(item.status)}</Badge>
                  {item.playable ? (
                    <Link className="btn btn-ghost" to={`/player/${item.id}`}>
                      Watch
                    </Link>
                  ) : (
                    <Link className="btn btn-ghost" to={`/torrents/${item.torrentId}`}>
                      Details
                    </Link>
                  )}
                  <Button onClick={() => setShareOpen(true)}>Share</Button>
                  <Button
                    variant="danger"
                    disabled={deletingTorrentId === item.torrentId}
                    onClick={() => setPendingDelete(item)}
                  >
                    {deletingTorrentId === item.torrentId ? "Deleting..." : "Delete torrent"}
                  </Button>
                </div>
              </div>
            </article>
          ))}
          {visibleItems.length === 0 ? (
            <EmptyState title="No media in filter">
              {loading ? "Loading media records." : "Search and filters collapse to this state when no videos match."}
            </EmptyState>
          ) : null}
        </div>

        <aside className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Operational states</div>
              <p className="muted">The library updates from torrent file status, HLS readiness, and direct playback support.</p>
            </div>
          </div>
          <div className="timeline-list">
            <div className="timeline-item">
              <Badge tone="ready">Ready</Badge>
              <div>
                <strong>{stats.ready} playable</strong>
                <p className="muted">HLS output or direct MP4 playback is available.</p>
              </div>
              <Progress value={items.length > 0 ? (stats.ready / items.length) * 100 : 0} />
            </div>
            <div className="timeline-item">
              <Badge tone="processing">Processing</Badge>
              <div>
                <strong>{stats.processing} active</strong>
                <p className="muted">Files are queued, downloading, or transcoding.</p>
              </div>
              <Progress value={items.length > 0 ? (stats.processing / items.length) * 100 : 0} />
            </div>
          </div>
        </aside>
      </div>

      <AddMagnetModal open={magnetOpen} onClose={() => setMagnetOpen(false)} onCreate={addTorrent} />
      <ShareModal open={shareOpen} onClose={() => setShareOpen(false)} />
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

function apiFileToLibraryItem(file: api.TorrentFile, torrent: api.Torrent): LibraryItem {
  const playable = (file.status === "done" && Boolean(file.hlsPath)) || file.directPlayable;
  const failed = file.status === "error";
  const status: LibraryStatus = failed ? "failed" : playable ? "ready" : "processing";
  const progress = playable ? 100 : Math.round(file.transcodingPercent || torrent.progressPercent || 0);
  const meta = [
    file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "Pending",
    codecLabel(file),
    file.processingMode ? file.processingMode.replace(/_/g, " ") : torrent.status,
  ];

  return {
    id: file.id,
    source: "api",
    torrentId: torrent.id,
    title: file.name,
    duration: formatDuration(file.durationSeconds || 0),
    meta,
    status,
    tags: [status, "recent"],
    progress,
    posterHue: hueFromID(file.id),
    playable,
    searchText: [file.name, torrent.name, file.status, torrent.status, ...meta].join(" ").toLowerCase(),
  };
}

function mockMediaToLibraryItem(item: (typeof mediaItems)[number]): LibraryItem {
  const status = item.status === "failed" ? "failed" : item.status === "ready" ? "ready" : "processing";
  return {
    id: item.id,
    source: "mock",
    torrentId: item.torrentId,
    title: item.title,
    duration: item.duration,
    meta: item.meta,
    status,
    tags: item.tags,
    progress: item.progress,
    posterHue: item.posterHue,
    playable: status === "ready",
    searchText: [item.title, item.status, ...item.meta].join(" ").toLowerCase(),
  };
}

function statusLabel(status: string) {
  return status.charAt(0).toUpperCase() + status.slice(1);
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

export function ShareModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [created, setCreated] = useState(false);
  const link = "https://streamize.local/s/nh-24h";

  async function copyLink() {
    setCreated(true);
    try {
      await navigator.clipboard.writeText(link);
    } catch {
      // Clipboard can be unavailable in automated or insecure contexts.
    }
  }

  return (
    <Modal
      title="Create share link"
      open={open}
      onClose={onClose}
      footer={
        <Button variant="primary" type="button" onClick={() => setCreated(true)}>
          Generate link
        </Button>
      }
    >
      <Field label="Expiration">
        <Select defaultValue="24h">
          <option value="24h">24 hours</option>
          <option value="7d">7 days</option>
          <option value="30d">30 days</option>
        </Select>
      </Field>
      <Field label="Share scope">
        <Select defaultValue="video">
          <option value="video">Single video</option>
          <option value="torrent">Full torrent</option>
        </Select>
      </Field>
      {created ? <div className="alert alert-success is-visible">Share created.</div> : null}
      <div className="share-link">
        <Input readOnly value={link} />
        <Button type="button" onClick={copyLink}>
          Copy
        </Button>
      </div>
    </Modal>
  );
}
