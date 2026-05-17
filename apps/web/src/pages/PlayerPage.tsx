import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Maximize2, Pause, Play, Volume2 } from "lucide-react";

import { Badge, Button, Progress, StatCard } from "../components/ui";
import { filesForTorrent, findMedia, torrentFiles } from "../lib/mock-data";

export function PlayerPage() {
  const { fileId } = useParams();
  const initialMedia = findMedia(fileId);
  const related = useMemo(() => {
    const files = filesForTorrent(initialMedia.torrentId);
    return files.length ? files : torrentFiles.slice(0, 3);
  }, [initialMedia.torrentId]);
  const [selectedFileId, setSelectedFileId] = useState(fileId || related[0]?.id);
  const selectedFile = related.find((file) => file.id === selectedFileId) || related[0];
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(34);

  return (
    <section className="content player-content">
      <div className="player-layout">
        <section className={`player-stage ${playing ? "is-playing" : ""}`} aria-label="Video player">
          <div className="player-frame">
            <div className="player-gradient">
              <span className="eyebrow">HLS Manifest</span>
              <strong>{selectedFile?.name || initialMedia.title}</strong>
              <p>Source-quality streaming surface with torrent health and subtitle controls nearby.</p>
            </div>
          </div>
          <div className="player-controls">
            <Button variant="primary" onClick={() => setPlaying((current) => !current)} aria-pressed={playing}>
              {playing ? <Pause size={16} /> : <Play size={16} />} {playing ? "Pause" : "Play"}
            </Button>
            <input
              className="range"
              type="range"
              min="0"
              max="100"
              value={position}
              onChange={(event) => setPosition(Number(event.target.value))}
              aria-label="Playback position"
            />
            <span className="mono">{position}%</span>
            <Button>
              <Volume2 size={16} /> Audio
            </Button>
            <Button>
              <Maximize2 size={16} /> Fullscreen
            </Button>
          </div>
        </section>

        <aside className="panel player-side">
          <div className="panel-header">
            <div>
              <div className="panel-title">Supported video files</div>
              <p className="muted">Switching files updates metadata and status without leaving playback.</p>
            </div>
          </div>
          <div className="file-list">
            {related.map((file) => (
              <button
                className={`file-row ${file.id === selectedFileId ? "active" : ""}`}
                key={file.id}
                onClick={() => setSelectedFileId(file.id)}
              >
                <span>
                  <strong>{file.name}</strong>
                  <small>
                    {file.size} · {file.codec}
                  </small>
                </span>
                <Badge tone={file.status}>{file.status}</Badge>
              </button>
            ))}
          </div>
          <div className="component-row">
            <Link className="btn btn-ghost" to={`/torrents/${initialMedia.torrentId}`}>
              Torrent detail
            </Link>
            <Button>Share</Button>
          </div>
        </aside>
      </div>

      <div className="stats-row">
        <StatCard label="Pieces" value="98%" detail="availability" />
        <StatCard label="Peers" value="18" detail="healthy swarm" />
        <StatCard label="Buffer" value="42s" detail="local HLS" />
        <StatCard label="Subtitles" value={String(selectedFile?.subtitles || 0)} detail="tracks detected" />
      </div>

      <div className="panel">
        <div className="timeline-list">
          <div className="timeline-item">
            <Badge tone="online">Direct</Badge>
            <div>
              <strong>{selectedFile?.codec || "H.264 / AAC"}</strong>
              <p className="muted">HLS manifest ready, source retained, previews generated.</p>
            </div>
            <Progress value={100} />
          </div>
          <div className="timeline-item">
            <Badge tone="ready">Subtitles</Badge>
            <div>
              <strong>English, Spanish, Japanese</strong>
              <p className="muted">Default track stays off until selected by the viewer.</p>
            </div>
            <Button>Manage</Button>
          </div>
        </div>
      </div>
    </section>
  );
}
