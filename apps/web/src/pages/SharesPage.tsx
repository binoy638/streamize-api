import { type FormEvent, useMemo, useState } from "react";

import { Badge, Button, EmptyState, Field, Input, Modal, Select, StatCard } from "../components/ui";
import { type Share, shares as initialShares } from "../lib/mock-data";

const filters = ["all", "online", "paused", "offline"];

export function SharesPage() {
  const [shares, setShares] = useState<Share[]>(initialShares);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return shares.filter((share) => {
      const matchesFilter = filter === "all" || share.status === filter;
      const matchesQuery = !normalized || [share.title, share.scope, share.url].join(" ").toLowerCase().includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, query, shares]);

  function revoke(id: string) {
    setShares((current) =>
      current.map((share) => (share.id === id ? { ...share, status: "offline" } : share)),
    );
  }

  function addShare(share: Share) {
    setShares((current) => [share, ...current]);
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Public sharing</span>
          <h1>Expiring links with scope, owner, and revoke controls visible.</h1>
          <p>Share links are treated as operational records with clear expiry and status states.</p>
        </div>
        <div className="header-actions">
          <Button variant="primary" onClick={() => setOpen(true)}>
            Create share
          </Button>
        </div>
      </div>

      <div className="stats-row">
        <StatCard label="Active shares" value="2" detail="publicly reachable" />
        <StatCard label="Expired" value="1" detail="hidden by default" />
        <StatCard label="Most recent" value="24h" detail="Neon Harbor" />
        <StatCard label="Owners" value="2" detail="admin, mira" />
      </div>

      <div className="toolbar">
        <div className="filters">
          {filters.map((status) => (
            <button
              key={status}
              className={`filter-chip ${filter === status ? "active" : ""}`}
              onClick={() => setFilter(status)}
            >
              {status === "all" ? "All" : status}
            </button>
          ))}
        </div>
        <label className="inline-search">
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter shares" />
        </label>
      </div>

      <div className="panel">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Share</th>
                <th>Scope</th>
                <th>Status</th>
                <th>Expires</th>
                <th>Owner</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((share) => (
                <tr key={share.id}>
                  <td>
                    <div className="table-title">
                      <strong>{share.title}</strong>
                      <span>{share.url}</span>
                    </div>
                  </td>
                  <td>{share.scope}</td>
                  <td>
                    <Badge tone={share.status}>{share.status}</Badge>
                  </td>
                  <td>{share.expiresAt}</td>
                  <td>{share.createdBy}</td>
                  <td>
                    <div className="row-actions">
                      <Button onClick={() => void navigator.clipboard?.writeText(share.url)}>Copy</Button>
                      <Button variant="danger" onClick={() => revoke(share.id)}>
                        Revoke
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {visible.length === 0 ? <EmptyState title="No shares">No links match this view.</EmptyState> : null}
      </div>

      <CreateShareModal open={open} onClose={() => setOpen(false)} onCreate={addShare} />
    </section>
  );
}

function CreateShareModal({
  open,
  onClose,
  onCreate,
}: {
  open: boolean;
  onClose: () => void;
  onCreate: (share: Share) => void;
}) {
  const [title, setTitle] = useState("Neon Harbor S02E04");
  const [scope, setScope] = useState<Share["scope"]>("Single video");
  const [expiration, setExpiration] = useState("24 hours");
  const [result, setResult] = useState("");

  function submit(event: FormEvent) {
    event.preventDefault();
    const id = `shr_${Date.now()}`;
    const url = `https://streamize.local/s/${title.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")}`;
    onCreate({
      id,
      title,
      scope,
      url,
      expiresAt: expiration,
      status: "online",
      createdBy: "admin",
    });
    setResult(url);
  }

  return (
    <Modal
      title="Create share link"
      open={open}
      onClose={onClose}
      footer={
        <Button variant="primary" type="submit" form="create-share-form">
          Generate link
        </Button>
      }
    >
      <form id="create-share-form" className="contents" onSubmit={submit}>
        <Field label="Title">
          <Input value={title} onChange={(event) => setTitle(event.target.value)} />
        </Field>
        <Field label="Expiration">
          <Select value={expiration} onChange={(event) => setExpiration(event.target.value)}>
            <option>24 hours</option>
            <option>7 days</option>
            <option>30 days</option>
          </Select>
        </Field>
        <Field label="Share scope">
          <Select value={scope} onChange={(event) => setScope(event.target.value as Share["scope"])}>
            <option>Single video</option>
            <option>Full torrent</option>
          </Select>
        </Field>
        {result ? (
          <>
            <div className="alert alert-success is-visible">Share created.</div>
            <div className="share-link">
              <Input readOnly value={result} />
              <Button type="button" onClick={() => void navigator.clipboard?.writeText(result)}>
                Copy
              </Button>
            </div>
          </>
        ) : null}
      </form>
    </Modal>
  );
}
