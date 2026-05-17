import { type CSSProperties, type FormEvent, useMemo, useState } from "react";
import { Link } from "react-router-dom";

import { Badge, Button, EmptyState, Field, Input, Modal, Progress, Select, StatCard, Textarea } from "../components/ui";
import { mediaItems } from "../lib/mock-data";

const filters = [
  { label: "All", value: "all" },
  { label: "Ready", value: "ready" },
  { label: "Processing", value: "processing" },
  { label: "Failed", value: "failed" },
  { label: "Recently added", value: "recent" },
];

export function LibraryPage() {
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");
  const [magnetOpen, setMagnetOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);

  const visibleItems = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return mediaItems.filter((item) => {
      const matchesFilter = filter === "all" || item.tags.includes(filter);
      const matchesQuery =
        !normalized ||
        [item.title, item.status, ...item.meta].join(" ").toLowerCase().includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, query]);

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Media Library</span>
          <h1>Ready to watch, still processing, or failed at a glance.</h1>
          <p>
            Cards prioritize status, playback readiness, and the next operational action instead of
            marketing-style artwork.
          </p>
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

      <div className="stats-row">
        <StatCard label="Ready videos" value="147" detail="+12 this week" />
        <StatCard label="Processing" value="18" detail="7 HLS jobs active" />
        <StatCard label="Failed" value="4" detail="needs retry" />
        <StatCard label="Storage" value="7.8 TB" detail="68% of /mnt/media" />
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
          placeholder="Filter visible cards"
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
                  {item.status === "ready" ? (
                    <Link className="btn btn-ghost" to={`/player/${item.id}`}>
                      Watch
                    </Link>
                  ) : item.status === "processing" ? (
                    <Link className="btn btn-ghost" to={`/torrents/${item.torrentId}`}>
                      Details
                    </Link>
                  ) : (
                    <Button onClick={() => setFilter("processing")}>Retry</Button>
                  )}
                  <Button onClick={() => setShareOpen(true)}>Share</Button>
                </div>
              </div>
            </article>
          ))}
          {visibleItems.length === 0 ? (
            <EmptyState title="No media in filter">
              Search and filters collapse to this state when no videos match.
            </EmptyState>
          ) : null}
        </div>

        <aside className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Operational states</div>
              <p className="muted">
                Empty, loading, error, and success states are part of the screen contract.
              </p>
            </div>
          </div>
          <EmptyState title="No media in filter">
            Search and filters collapse to this state when no videos match.
          </EmptyState>
          <div className="skeleton-grid">
            <div className="skeleton" />
            <div className="skeleton" />
            <div className="skeleton" />
          </div>
          <div className="alert alert-error is-visible">
            Thumbnail extraction failed for North Cache. Retry queued manually.
          </div>
        </aside>
      </div>

      <AddMagnetModal open={magnetOpen} onClose={() => setMagnetOpen(false)} />
      <ShareModal open={shareOpen} onClose={() => setShareOpen(false)} />
    </section>
  );
}

function statusLabel(status: string) {
  return status.charAt(0).toUpperCase() + status.slice(1);
}

function AddMagnetModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [magnet, setMagnet] = useState("");
  const [category, setCategory] = useState("/mnt/media/movies");
  const [message, setMessage] = useState("");
  const [tone, setTone] = useState<"success" | "error">("success");
  const [busy, setBusy] = useState(false);

  function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    window.setTimeout(() => {
      setBusy(false);
      if (!magnet.trim().match(/^magnet:\?xt=/)) {
        setTone("error");
        setMessage("Paste a valid magnet URL beginning with magnet:?xt=.");
        return;
      }
      setTone("success");
      setMessage(`Torrent added to ${category || "/mnt/media"}. Metadata fetch has started.`);
    }, 350);
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
          <Button variant="primary" type="submit" form="add-magnet-form" disabled={busy}>
            {busy ? "Adding..." : "Add to queue"}
          </Button>
        </>
      }
    >
      <form id="add-magnet-form" className="contents" onSubmit={submit}>
        {message ? <div className={`alert alert-${tone} is-visible`}>{message}</div> : null}
        <Field label="Magnet URL">
          <Textarea
            value={magnet}
            onChange={(event) => setMagnet(event.target.value)}
            placeholder="magnet:?xt=urn:btih:..."
          />
        </Field>
        <Field label="Destination / category">
          <Input value={category} onChange={(event) => setCategory(event.target.value)} />
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
