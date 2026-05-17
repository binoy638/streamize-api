import { type CSSProperties, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { Badge, Button, EmptyState, Progress, StatCard } from "../components/ui";
import { filesForTorrent, findTorrent, jobs, shares } from "../lib/mock-data";

const tabs = ["overview", "files", "jobs", "subtitles", "shares"];

export function TorrentDetailPage() {
  const { id } = useParams();
  const torrent = findTorrent(id);
  const files = useMemo(() => filesForTorrent(torrent.id), [torrent.id]);
  const [activeTab, setActiveTab] = useState("overview");

  return (
    <section className="content">
      <div className="detail-hero">
        <div className="poster detail-poster" style={{ "--poster-hue": 235 } as CSSProperties}>
          <span className="poster-label">{torrent.name.slice(0, 18)}</span>
        </div>
        <div className="detail-copy">
          <span className="eyebrow">Torrent detail</span>
          <h1>{torrent.name}</h1>
          <p>
            Files, processing jobs, subtitles, and shares stay attached to the torrent record so
            operations are visible without hopping across screens.
          </p>
          <div className="component-row">
            <Badge tone={torrent.status}>{torrent.status}</Badge>
            <span className="mono">{torrent.size}</span>
            <span>{torrent.peers} peers</span>
          </div>
          <div className="progress-row wide">
            <Progress value={torrent.progress} />
            <span>{torrent.progress}%</span>
          </div>
        </div>
      </div>

      <div className="stats-row">
        <StatCard label="Files" value={String(files.length || 1)} detail="video candidates" />
        <StatCard label="Jobs" value="4" detail="1 failed, 2 running" />
        <StatCard label="Subtitles" value="6" detail="detected or extracted" />
        <StatCard label="Retention" value="Keep" detail="originals retained" />
      </div>

      <div className="tabs" role="tablist">
        {tabs.map((tab) => (
          <button
            key={tab}
            className={activeTab === tab ? "active" : ""}
            onClick={() => setActiveTab(tab)}
            role="tab"
            aria-selected={activeTab === tab}
          >
            {tab}
          </button>
        ))}
      </div>

      <div className="panel">
        {activeTab === "overview" ? (
          <div className="overview-grid">
            <Info title="Info hash" value="a91d...72cf" />
            <Info title="qBittorrent hash" value="QBT-7A91D2" />
            <Info title="Added" value={torrent.addedAt} />
            <Info title="ETA" value={torrent.eta} />
          </div>
        ) : null}

        {activeTab === "files" ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>File</th>
                  <th>Status</th>
                  <th>Transcode</th>
                  <th>Codec</th>
                  <th>Subtitles</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {files.map((file) => (
                  <tr key={file.id}>
                    <td>
                      <div className="table-title">
                        <strong>{file.name}</strong>
                        <span>{file.size}</span>
                      </div>
                    </td>
                    <td>
                      <Badge tone={file.status}>{file.status}</Badge>
                    </td>
                    <td className="table-progress">
                      <Progress value={file.progress} />
                      <span>{file.progress}%</span>
                    </td>
                    <td>{file.codec}</td>
                    <td>{file.subtitles}</td>
                    <td>
                      <Link className="btn btn-ghost" to={`/player/${file.id}`}>
                        Open
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {files.length === 0 ? <EmptyState title="No files">Metadata sync has not found files yet.</EmptyState> : null}
          </div>
        ) : null}

        {activeTab === "jobs" ? (
          <div className="timeline-list">
            {jobs.slice(0, 4).map((job) => (
              <div className="timeline-item" key={job.id}>
                <Badge tone={job.status}>{job.status}</Badge>
                <div>
                  <strong>{job.type}</strong>
                  <p className="muted">
                    {job.target} · {job.worker} · {job.updatedAt}
                  </p>
                </div>
                <Progress value={job.progress} />
              </div>
            ))}
          </div>
        ) : null}

        {activeTab === "subtitles" ? (
          <div className="overview-grid">
            <Info title="English" value="Embedded SRT extracted" />
            <Info title="Spanish" value="Uploaded VTT" />
            <Info title="Japanese" value="Queued for extraction" />
            <Info title="Default" value="English forced off" />
          </div>
        ) : null}

        {activeTab === "shares" ? (
          <div className="timeline-list">
            {shares.slice(0, 2).map((share) => (
              <div className="timeline-item" key={share.id}>
                <Badge tone={share.status}>{share.status}</Badge>
                <div>
                  <strong>{share.title}</strong>
                  <p className="muted">
                    {share.scope} · expires {share.expiresAt}
                  </p>
                </div>
                <Button>Copy</Button>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function Info({ title, value }: { title: string; value: string }) {
  return (
    <div className="info-tile">
      <span>{title}</span>
      <strong>{value}</strong>
    </div>
  );
}
