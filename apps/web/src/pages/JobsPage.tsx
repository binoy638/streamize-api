import { useCallback, useEffect, useMemo, useState } from "react";

import * as api from "../lib/api";
import { Badge, Button, EmptyState, Input, Progress, StatCard, StatSkeletonGrid, TableSkeletonRows } from "../components/ui";
import { formatDate } from "../lib/format";

const filters = ["all", "queued", "running", "succeeded", "failed", "canceled"];
const pollableStatuses = new Set(["queued", "running"]);

type LoadJobsOptions = {
  showLoading?: boolean;
};

type JobRow = {
  id: string;
  type: string;
  typeTone: string;
  target: string;
  status: api.JobStatus;
  progress: number;
  attempts: string;
  worker: string;
  startedAt: string;
  finishedAt: string;
  updatedAt: string;
  error?: string;
  searchText: string;
};

type Notice = {
  tone: "success" | "warn";
  text: string;
};

export function JobsPage() {
  const [jobs, setJobs] = useState<JobRow[]>([]);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [busyJobId, setBusyJobId] = useState("");

  const loadJobs = useCallback(async ({ showLoading = true }: LoadJobsOptions = {}) => {
    if (showLoading) {
      setLoading(true);
    }
    try {
      const result = await api.listJobs();
      setJobs(result.map(apiJobToRow));
      setError("");
    } catch (err) {
      if (showLoading) {
        setJobs([]);
      }
      setError(err instanceof Error ? err.message : "Unable to load jobs");
    } finally {
      if (showLoading) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    void loadJobs();
  }, [loadJobs]);

  useEffect(() => {
    const hasActiveJob = jobs.some((job) => pollableStatuses.has(job.status));
    if (!hasActiveJob) {
      return;
    }

    const intervalID = window.setInterval(() => {
      void loadJobs({ showLoading: false });
    }, 5000);

    return () => window.clearInterval(intervalID);
  }, [jobs, loadJobs]);

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return jobs.filter((job) => {
      const matchesFilter = filter === "all" || job.status === filter;
      const matchesQuery = !normalized || job.searchText.includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, jobs, query]);

  const stats = useMemo(() => {
    const count = (status: string) => jobs.filter((job) => job.status === status).length;
    return {
      running: count("running"),
      queued: count("queued"),
      succeeded: count("succeeded"),
      failed: count("failed"),
    };
  }, [jobs]);

  async function retryJob(job: JobRow) {
    setBusyJobId(job.id);
    try {
      const updated = await api.retryJob(job.id);
      setJobs((current) => current.map((item) => (item.id === job.id ? apiJobToRow(updated) : item)));
      setNotice({ tone: "success", text: "Job queued for retry." });
    } catch (err) {
      setNotice({ tone: "warn", text: err instanceof Error ? err.message : "Unable to retry job." });
    } finally {
      setBusyJobId("");
    }
  }

  async function cancelJob(job: JobRow) {
    setBusyJobId(job.id);
    try {
      const updated = await api.cancelJob(job.id);
      setJobs((current) => current.map((item) => (item.id === job.id ? apiJobToRow(updated) : item)));
      setNotice({ tone: "success", text: "Job canceled." });
    } catch (err) {
      setNotice({ tone: "warn", text: err instanceof Error ? err.message : "Unable to cancel job." });
    } finally {
      setBusyJobId("");
    }
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Processing queue</span>
          <h1>ffprobe, HLS, subtitles, previews, and cleanup jobs stay inspectable.</h1>
          <p>Queue state is treated as first-class product UI instead of hidden server logs.</p>
        </div>
      </div>

      {error ? (
        <div className="alert alert-error is-visible">Unable to load jobs: {error}</div>
      ) : null}
      {notice ? <div className={`alert alert-${notice.tone} is-visible`}>{notice.text}</div> : null}

      {loading ? (
        <StatSkeletonGrid />
      ) : (
        <div className="stats-row">
          <StatCard label="Running" value={String(stats.running)} detail="workers claimed leases" />
          <StatCard label="Queued" value={String(stats.queued)} detail="available now" />
          <StatCard label="Succeeded" value={String(stats.succeeded)} detail="completed jobs" />
          <StatCard label="Failed" value={String(stats.failed)} detail="needs retry" />
        </div>
      )}

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
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter jobs" />
        </label>
      </div>

      <div className="panel">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Job</th>
                <th>Status</th>
                <th>Progress</th>
                <th>Started</th>
                <th>Finished</th>
                <th>Attempts</th>
                <th>Worker</th>
                <th>Updated</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {loading ? <TableSkeletonRows rows={5} columns={9} /> : null}
              {visible.map((job) => (
                <tr key={job.id}>
                  <td>
                    <div className="table-title">
                      <Badge tone={`job-type ${job.typeTone}`}>{job.type}</Badge>
                      <span>{job.target}</span>
                    </div>
                  </td>
                  <td>
                    <Badge tone={job.status}>{job.status}</Badge>
                  </td>
                  <td className="table-progress">
                    <Progress value={job.progress} />
                    <span>{job.progress}%</span>
                  </td>
                  <td>{job.startedAt}</td>
                  <td>{job.finishedAt}</td>
                  <td>{job.attempts}</td>
                  <td>{job.worker}</td>
                  <td>{job.updatedAt}</td>
                  <td>
                    <div className="row-actions">
                      <Button disabled={!canRetryJob(job) || busyJobId === job.id} onClick={() => void retryJob(job)}>
                        {busyJobId === job.id ? "Working..." : job.status === "succeeded" ? "Reprocess" : "Retry"}
                      </Button>
                      <Button variant="danger" disabled={!canCancelJob(job) || busyJobId === job.id} onClick={() => void cancelJob(job)}>
                        Cancel
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {!loading && visible.length === 0 ? (
          <EmptyState title="No jobs">{query || filter !== "all" ? "No queue items match this view." : "No processing jobs have been queued yet."}</EmptyState>
        ) : null}
      </div>
    </section>
  );
}

function apiJobToRow(job: api.Job): JobRow {
  const target = job.target || job.torrentFileId || "Unattached job";
  const worker = workerLabel(job.lockedBy, job.status);
  const progress = progressForAPIJob(job);
  const attempts = `${job.attempts}/${job.maxAttempts}`;
  const type = jobTypeLabel(job.type);
  const typeTone = jobTypeTone(job.type);

  return {
    id: job.id,
    type,
    typeTone,
    target,
    status: job.status,
    progress,
    attempts,
    worker,
    startedAt: formatJobTime(job.startedAt, "Not started"),
    finishedAt: formatJobTime(job.finishedAt, job.status === "running" ? "Running" : "Not finished"),
    updatedAt: formatDate(job.updatedAt),
    error: job.lastError,
    searchText: buildSearchText(type, job.type, target, job.status, worker, attempts, job.lastError),
  };
}

function progressForAPIJob(job: api.Job): number {
  if (job.status === "succeeded") {
    return 100;
  }
  if (job.status === "queued" || job.status === "canceled") {
    return 0;
  }
  if (!Number.isFinite(job.progressPercent)) {
    return 0;
  }

  return Math.round(Math.max(0, Math.min(job.progressPercent, 100)));
}

function canRetryJob(job: JobRow): boolean {
  return job.status === "failed" || job.status === "canceled" || job.status === "succeeded";
}

function canCancelJob(job: JobRow): boolean {
  return job.status === "queued" || job.status === "failed";
}

function buildSearchText(...values: Array<string | undefined>) {
  return values.filter(Boolean).join(" ").toLowerCase();
}

function jobTypeLabel(type: string): string {
  switch (type) {
    case "metadata_identify":
      return "Metadata identify";
    case "hls_transcode":
    case "hls-generate":
      return "HLS transcode";
    case "subtitle_extract":
    case "subtitle-extract":
      return "Subtitle extraction";
    case "sprite_generate":
    case "thumbnail-sprite":
      return "Sprite generation";
    case "ffprobe":
      return "Media probe";
    default:
      return type.replace(/[_-]/g, " ");
  }
}

function jobTypeTone(type: string): string {
  switch (type) {
    case "metadata_identify":
      return "job-metadata";
    case "hls_transcode":
    case "hls-generate":
      return "job-hls";
    case "subtitle_extract":
    case "subtitle-extract":
      return "job-subtitle";
    case "sprite_generate":
    case "thumbnail-sprite":
      return "job-sprite";
    case "ffprobe":
      return "job-probe";
    default:
      return "job-other";
  }
}

function workerLabel(lockedBy: string | undefined, status: api.JobStatus): string {
  if (lockedBy) {
    return lockedBy;
  }

  switch (status) {
    case "running":
      return "claimed";
    case "queued":
      return "unclaimed";
    case "succeeded":
      return "completed";
    case "failed":
      return "failed";
    case "canceled":
      return "canceled";
  }
}

function formatJobTime(value: string | undefined, fallback: string): string {
  if (!value) {
    return fallback;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
