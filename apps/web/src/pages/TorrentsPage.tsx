import { useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { Badge, Button, EmptyState, Input, Progress, StatCard } from "../components/ui";
import { type Torrent, torrents as initialTorrents } from "../lib/mock-data";

const statusFilters = ["all", "downloading", "processing", "done", "paused", "error"];

export function TorrentsPage() {
  const [torrents, setTorrents] = useState<Torrent[]>(initialTorrents);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return torrents.filter((torrent) => {
      const matchesFilter = filter === "all" || torrent.status === filter;
      const matchesQuery =
        !normalized ||
        [torrent.name, torrent.status, torrent.size, torrent.eta].join(" ").toLowerCase().includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, query, torrents]);

  function updateStatus(id: string, status: Torrent["status"]) {
    setTorrents((current) =>
      current.map((torrent) =>
        torrent.id === id
          ? {
              ...torrent,
              status,
              eta: status === "paused" ? "Paused" : status === "downloading" ? "18 min" : torrent.eta,
            }
          : torrent,
      ),
    );
  }

  function removeTorrent(id: string) {
    setTorrents((current) => current.filter((torrent) => torrent.id !== id));
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Torrent orchestration</span>
          <h1>Transfers, queue health, and file readiness in one operational table.</h1>
          <p>Local actions update prototype state now and can map directly to qBittorrent controls later.</p>
        </div>
      </div>

      <div className="stats-row">
        <StatCard label="Downloading" value="2" detail="32 MB/s aggregate" />
        <StatCard label="Processing" value="2" detail="HLS and thumbnails" />
        <StatCard label="Done" value="3" detail="seeding or retained" />
        <StatCard label="Errors" value="1" detail="manual attention" />
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
                <th>Speed</th>
                <th>ETA</th>
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
                        {torrent.size} · ratio {torrent.ratio}
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
                  <td className="mono">{torrent.speed}</td>
                  <td>{torrent.eta}</td>
                  <td>{torrent.peers}</td>
                  <td>
                    <div className="row-actions">
                      <Button onClick={() => updateStatus(torrent.id, "paused")}>Pause</Button>
                      <Button onClick={() => updateStatus(torrent.id, "downloading")}>Resume</Button>
                      <Button onClick={() => updateStatus(torrent.id, "queued")}>Retry</Button>
                      <Button variant="danger" onClick={() => removeTorrent(torrent.id)}>
                        Delete
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {visible.length === 0 ? <EmptyState title="No torrents">No records match this filter.</EmptyState> : null}
      </div>
    </section>
  );
}

function label(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
