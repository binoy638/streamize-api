import { useCallback, useEffect, useMemo, useState } from "react";

import { Badge, Button, EmptyState, Input, StatCard, StatSkeletonGrid, TableSkeletonRows } from "../components/ui";
import { CreateShareModal } from "../components/CreateShareModal";
import * as api from "../lib/api";

type ShareFilter = "all" | "active" | "expired";

const FILTERS: ShareFilter[] = ["all", "active", "expired"];

export function SharesPage() {
  const [shares, setShares] = useState<api.ShareSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [filter, setFilter] = useState<ShareFilter>("all");
  const [query, setQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [revokingId, setRevokingId] = useState("");

  const loadShares = useCallback(async () => {
    try {
      const records = await api.listShares();
      setShares(records);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load shares.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadShares();
  }, [loadShares]);

  const stats = useMemo(() => {
    const active = shares.filter((share) => !share.expired).length;
    return { active, expired: shares.length - active, total: shares.length };
  }, [shares]);

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return shares.filter((share) => {
      const matchesFilter =
        filter === "all" || (filter === "active" ? !share.expired : share.expired);
      const matchesQuery = !normalized || [share.title, share.scope, share.url].join(" ").toLowerCase().includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, query, shares]);

  async function revoke(share: api.ShareSummary) {
    setRevokingId(share.id);
    setNotice("");
    try {
      await api.revokeShare(share.id);
      setShares((current) => current.filter((record) => record.id !== share.id));
      setNotice(`Revoked the share link for "${share.title}".`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to revoke share.");
    } finally {
      setRevokingId("");
    }
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Public sharing</span>
          <h1>Expiring links anyone can watch — no account required.</h1>
          <p>Create, copy, and revoke share links. Expired links stop working immediately.</p>
        </div>
        <div className="header-actions">
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            Create share
          </Button>
        </div>
      </div>

      {error ? <div className="alert alert-error is-visible">{error}</div> : null}
      {notice ? <div className="alert alert-success is-visible">{notice}</div> : null}

      {loading ? (
        <StatSkeletonGrid count={3} />
      ) : (
        <div className="stats-row">
          <StatCard label="Active shares" value={String(stats.active)} detail="publicly reachable" />
          <StatCard label="Expired" value={String(stats.expired)} detail="no longer working" />
          <StatCard label="Total" value={String(stats.total)} detail="links created" />
        </div>
      )}

      <div className="toolbar">
        <div className="filters">
          {FILTERS.map((status) => (
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
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {loading ? <TableSkeletonRows rows={4} columns={5} /> : null}
              {visible.map((share) => (
                <tr key={share.id}>
                  <td>
                    <div className="table-title">
                      <strong>{share.title}</strong>
                      <span>{share.url}</span>
                    </div>
                  </td>
                  <td>{scopeLabel(share)}</td>
                  <td>
                    <Badge tone={share.expired ? "offline" : "online"}>
                      {share.expired ? "Expired" : "Active"}
                    </Badge>
                  </td>
                  <td>{formatExpiry(share.expiresAt)}</td>
                  <td>
                    <div className="row-actions">
                      <Button onClick={() => void navigator.clipboard?.writeText(share.url)}>Copy</Button>
                      <Button
                        variant="danger"
                        disabled={revokingId === share.id}
                        onClick={() => void revoke(share)}
                      >
                        {revokingId === share.id ? "Revoking…" : "Revoke"}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!loading && visible.length === 0 ? (
          <EmptyState title="No shares">
            {shares.length === 0
              ? "Create a share link to let anyone watch a video without signing in."
              : "No links match this view."}
          </EmptyState>
        ) : null}
      </div>

      <CreateShareModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={() => {
          setNotice("Share link created and copied to clipboard.");
          void loadShares();
        }}
      />
    </section>
  );
}

function scopeLabel(share: api.ShareSummary): string {
  if (share.scope === "torrent") {
    return `Full torrent · ${share.fileCount} file${share.fileCount === 1 ? "" : "s"}`;
  }
  return "Single video";
}

function formatExpiry(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}
