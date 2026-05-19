import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import * as api from "../lib/api";
import { Badge, Button, EmptyState, Field, Input, Modal, Progress, StatCard, Textarea } from "../components/ui";
import { formatBytes, formatDate, formatTransferRate } from "../lib/format";
import { type Torrent as MockTorrent, torrents as initialTorrents } from "../lib/mock-data";

const statusFilters = ["all", "added", "queued", "downloading", "processing", "done", "paused", "error"];
const pollableStatuses = new Set(["added", "queued", "downloading", "processing"]);

type LoadTorrentsOptions = {
  showLoading?: boolean;
  fallbackToMock?: boolean;
};

type TorrentRow = {
  id: string;
  source: "api" | "mock";
  name: string;
  status: api.TorrentStatus | MockTorrent["status"];
  size: string;
  progress: number;
  speed: string;
  eta: string;
  peers: string;
  ratio: string;
  addedAt: string;
  searchText: string;
};

type Notice = {
  tone: "success" | "warn";
  text: string;
};

export function TorrentsPage() {
  const [torrents, setTorrents] = useState<TorrentRow[]>([]);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [usingMock, setUsingMock] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [deletingId, setDeletingId] = useState("");

  const loadTorrents = useCallback(async ({ showLoading = true, fallbackToMock = true }: LoadTorrentsOptions = {}) => {
    if (showLoading) {
      setLoading(true);
    }
    try {
      const result = await api.listTorrents();
      setTorrents(result.map(apiTorrentToRow));
      setUsingMock(false);
      setError("");
    } catch (err) {
      if (fallbackToMock) {
        setTorrents(initialTorrents.map(mockTorrentToRow));
        setUsingMock(true);
        setError(err instanceof Error ? err.message : "Unable to load torrents");
      }
    } finally {
      if (showLoading) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    void loadTorrents();
  }, [loadTorrents]);

  useEffect(() => {
    if (usingMock) {
      return;
    }

    const hasActiveAPITorrent = torrents.some((torrent) => torrent.source === "api" && pollableStatuses.has(torrent.status));
    if (!hasActiveAPITorrent) {
      return;
    }

    const intervalID = window.setInterval(() => {
      void loadTorrents({ showLoading: false, fallbackToMock: false });
    }, 5000);

    return () => window.clearInterval(intervalID);
  }, [loadTorrents, torrents, usingMock]);

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return torrents.filter((torrent) => {
      const matchesFilter = filter === "all" || torrent.status === filter;
      const matchesQuery = !normalized || torrent.searchText.includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, query, torrents]);

  const stats = useMemo(() => {
    const count = (statuses: string[]) => torrents.filter((torrent) => statuses.includes(torrent.status)).length;
    return {
      active: count(["added", "queued", "downloading"]),
      processing: count(["processing"]),
      done: count(["done"]),
      errors: count(["error"]),
    };
  }, [torrents]);

  function updateStatus(id: string, status: MockTorrent["status"]) {
    setTorrents((current) =>
      current.map((torrent) =>
        torrent.id === id && torrent.source === "mock"
          ? {
              ...torrent,
              status,
              eta: status === "paused" ? "Paused" : status === "downloading" ? "18 min" : torrent.eta,
              searchText: buildSearchText(torrent.name, status, torrent.size, torrent.eta),
            }
          : torrent,
      ),
    );
  }

  async function removeTorrent(torrent: TorrentRow) {
    if (torrent.source === "mock") {
      setTorrents((current) => current.filter((item) => item.id !== torrent.id));
      setNotice({ tone: "warn", text: "Prototype torrent removed locally." });
      return;
    }

    setDeletingId(torrent.id);
    try {
      await api.deleteTorrent(torrent.id);
      setTorrents((current) => current.filter((item) => item.id !== torrent.id));
      setNotice({ tone: "success", text: "Torrent deleted from the API." });
    } catch (err) {
      setNotice({ tone: "warn", text: err instanceof Error ? err.message : "Unable to delete torrent." });
    } finally {
      setDeletingId("");
    }
  }

  async function addTorrent(input: api.CreateTorrentInput) {
    setFilter("all");
    setQuery("");
    if (usingMock) {
      const now = new Date().toISOString();
      const local: TorrentRow = {
        id: `tor_${Date.now()}`,
        source: "mock",
        name: input.name?.trim() || "New magnet",
        status: "queued",
        size: "Pending",
        progress: 0,
        speed: "0 B/s",
        eta: "Queued",
        peers: "0",
        ratio: "0.0",
        addedAt: formatDate(now),
        searchText: buildSearchText(input.name || "New magnet", "queued", "Pending", "Queued"),
      };
      setTorrents((current) => [local, ...current]);
      setNotice({ tone: "warn", text: "Prototype torrent added locally. Sign in through the API to persist magnet submissions." });
      return;
    }

    const created = await api.createTorrent(input);
    setTorrents((current) => [apiTorrentToRow(created), ...current]);
    setNotice({ tone: "success", text: "Magnet added and recorded by the API." });
  }

  async function refreshAfterSubmissionFailure(message: string) {
    setFilter("all");
    setQuery("");
    await loadTorrents();
    setNotice({ tone: "warn", text: `${message}. The torrent was recorded with Error status.` });
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Torrent orchestration</span>
          <h1>Transfers, queue health, and file readiness in one operational table.</h1>
          <p>Submit magnet links to qBittorrent and track the torrent records persisted by the API.</p>
        </div>
        <div className="header-actions">
          <Button variant="primary" onClick={() => setOpen(true)}>
            Add magnet
          </Button>
        </div>
      </div>

      {error ? (
        <div className="alert alert-warn is-visible">
          Using prototype torrents because the torrent API did not respond: {error}
        </div>
      ) : null}
      {notice ? <div className={`alert alert-${notice.tone} is-visible`}>{notice.text}</div> : null}

      <div className="stats-row">
        <StatCard label="Active" value={String(stats.active)} detail={usingMock ? "prototype queue" : "added or queued"} />
        <StatCard label="Processing" value={String(stats.processing)} detail="HLS and thumbnails" />
        <StatCard label="Done" value={String(stats.done)} detail="seeding or retained" />
        <StatCard label="Errors" value={String(stats.errors)} detail="manual attention" />
      </div>

      <div className="filters">
        {statusFilters.map((status) => (
          <button
            key={status}
            className={`filter-chip ${filter === status ? "active" : ""}`}
            onClick={() => setFilter(status)}
          >
            {status === "all" ? "All" : label(status)}
          </button>
        ))}
      </div>

      <label className="inline-search">
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Filter torrent records"
        />
      </label>

      <div className="panel">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Torrent</th>
                <th>Status</th>
                <th>Progress</th>
                <th>Size</th>
                <th>Added</th>
                <th>Peers</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((torrent) => (
                <tr key={torrent.id}>
                  <td>
                    <Link className="table-title" to={`/torrents/${torrent.id}`}>
                      <strong>{torrent.name}</strong>
                      <span>
                        {torrent.speed} · ratio {torrent.ratio}
                      </span>
                    </Link>
                  </td>
                  <td>
                    <Badge tone={torrent.status}>{label(torrent.status)}</Badge>
                  </td>
                  <td className="table-progress">
                    <Progress value={torrent.progress} />
                    <span>{torrent.progress}%</span>
                  </td>
                  <td className="mono">{torrent.size}</td>
                  <td>{torrent.addedAt}</td>
                  <td>{torrent.peers}</td>
                  <td>
                    <div className="row-actions">
                      <Button disabled={torrent.source === "api"} onClick={() => updateStatus(torrent.id, "paused")}>
                        Pause
                      </Button>
                      <Button disabled={torrent.source === "api"} onClick={() => updateStatus(torrent.id, "downloading")}>
                        Resume
                      </Button>
                      <Button disabled={torrent.source === "api"} onClick={() => updateStatus(torrent.id, "queued")}>
                        Retry
                      </Button>
                      <Button variant="danger" disabled={deletingId === torrent.id} onClick={() => void removeTorrent(torrent)}>
                        {deletingId === torrent.id ? "Deleting..." : "Delete"}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {visible.length === 0 ? (
          <EmptyState title="No torrents">{loading ? "Loading torrent records." : "No records match this filter."}</EmptyState>
        ) : null}
      </div>

      <AddMagnetModal
        open={open}
        onClose={() => setOpen(false)}
        onCreate={addTorrent}
        onRecordedFailure={refreshAfterSubmissionFailure}
        usingMock={usingMock}
      />
    </section>
  );
}

function AddMagnetModal({
  open,
  onClose,
  onCreate,
  onRecordedFailure,
  usingMock,
}: {
  open: boolean;
  onClose: () => void;
  onCreate: (input: api.CreateTorrentInput) => Promise<void>;
  onRecordedFailure: (message: string) => Promise<void>;
  usingMock: boolean;
}) {
  const [magnetUri, setMagnetUri] = useState("");
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setMessage("");
    if (!magnetUri.trim().startsWith("magnet:?")) {
      setMessage("Enter a valid magnet URI.");
      return;
    }

    setBusy(true);
    try {
      await onCreate({ magnetUri: magnetUri.trim(), name: name.trim() || undefined });
      setMagnetUri("");
      setName("");
      onClose();
    } catch (error) {
      if (error instanceof api.ApiError && error.status === 502) {
        await onRecordedFailure(error.message);
        setMagnetUri("");
        setName("");
        onClose();
        return;
      }
      setMessage(error instanceof Error ? error.message : "Unable to add magnet");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title="Add magnet"
      open={open}
      onClose={onClose}
      footer={
        <>
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" form="add-magnet-form" disabled={busy}>
            {busy ? "Adding..." : usingMock ? "Add prototype row" : "Add magnet"}
          </Button>
        </>
      }
    >
      <form id="add-magnet-form" className="contents" onSubmit={submit}>
        {message ? <div className={`alert ${message === "Magnet added." ? "alert-success" : "alert-error"} is-visible`}>{message}</div> : null}
        <Field label="Magnet URI">
          <Textarea value={magnetUri} onChange={(event) => setMagnetUri(event.target.value)} rows={5} placeholder="magnet:?xt=urn:btih:..." />
        </Field>
        <Field label="Display name">
          <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="Optional" />
        </Field>
      </form>
    </Modal>
  );
}

function apiTorrentToRow(torrent: api.Torrent): TorrentRow {
  const name = torrent.name || torrent.infoHash || torrent.id;
  const size = torrent.sizeBytes > 0 ? formatBytes(torrent.sizeBytes) : "Pending";
  const eta = etaForTorrent(torrent);
  const progress = progressForTorrent(torrent);
  const speed = transferRateForTorrent(torrent);
  const ratio = Number.isFinite(torrent.ratio) ? torrent.ratio.toFixed(1) : "0.0";

  return {
    id: torrent.id,
    source: "api",
    name,
    status: torrent.status,
    size,
    progress,
    speed,
    eta,
    peers: String(torrent.peers ?? 0),
    ratio,
    addedAt: formatDate(torrent.createdAt),
    searchText: buildSearchText(name, torrent.status, size, eta, speed, ratio, torrent.infoHash, torrent.qbittorrentHash, torrent.errorMessage),
  };
}

function mockTorrentToRow(torrent: MockTorrent): TorrentRow {
  return {
    id: torrent.id,
    source: "mock",
    name: torrent.name,
    status: torrent.status,
    size: torrent.size,
    progress: torrent.progress,
    speed: torrent.speed,
    eta: torrent.eta,
    peers: String(torrent.peers),
    ratio: torrent.ratio.toFixed(1),
    addedAt: torrent.addedAt,
    searchText: buildSearchText(torrent.name, torrent.status, torrent.size, torrent.eta),
  };
}

function progressForStatus(status: api.TorrentStatus): number {
  switch (status) {
    case "done":
      return 100;
    case "processing":
      return 75;
    case "downloading":
      return 35;
    case "error":
    case "added":
    case "queued":
    case "paused":
      return 0;
  }
}

function progressForTorrent(torrent: api.Torrent): number {
  if (Number.isFinite(torrent.progressPercent)) {
    return Math.round(Math.max(0, Math.min(torrent.progressPercent, 100)));
  }

  return progressForStatus(torrent.status);
}

function etaForTorrent(torrent: api.Torrent) {
  if (torrent.status === "done") {
    return "Complete";
  }
  if (torrent.status === "error") {
    return "Needs retry";
  }
  if (torrent.status === "paused") {
    return "Paused";
  }
  if (Number.isFinite(torrent.etaSeconds) && torrent.etaSeconds >= 0) {
    return formatDuration(torrent.etaSeconds);
  }

  switch (torrent.status) {
    case "processing":
      return "Processing";
    case "downloading":
      return "Downloading";
    default:
      return "Queued";
  }
}

function transferRateForTorrent(torrent: api.Torrent) {
  const download = formatTransferRate(torrent.downloadSpeedBytes ?? 0);
  const uploadBytes = torrent.uploadSpeedBytes ?? 0;
  if (uploadBytes > 0) {
    return `DL ${download} / UL ${formatTransferRate(uploadBytes)}`;
  }

  return download;
}

function formatDuration(totalSeconds: number) {
  const seconds = Math.max(0, Math.round(totalSeconds));
  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
}

function buildSearchText(...values: Array<string | undefined>) {
  return values.filter(Boolean).join(" ").toLowerCase();
}

function label(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
