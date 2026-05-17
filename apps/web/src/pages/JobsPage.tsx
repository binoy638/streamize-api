import { useMemo, useState } from "react";

import { Badge, Button, EmptyState, Input, Progress, StatCard } from "../components/ui";
import { type Job, jobs as initialJobs } from "../lib/mock-data";

const filters = ["all", "queued", "running", "succeeded", "failed"];

export function JobsPage() {
  const [jobs, setJobs] = useState<Job[]>(initialJobs);
  const [filter, setFilter] = useState("all");
  const [query, setQuery] = useState("");

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return jobs.filter((job) => {
      const matchesFilter = filter === "all" || job.status === filter;
      const matchesQuery = !normalized || [job.type, job.target, job.worker].join(" ").toLowerCase().includes(normalized);
      return matchesFilter && matchesQuery;
    });
  }, [filter, jobs, query]);

  function updateJob(id: string, status: Job["status"]) {
    setJobs((current) =>
      current.map((job) =>
        job.id === id
          ? {
              ...job,
              status,
              progress: status === "queued" ? 0 : job.progress,
              updatedAt: status === "queued" ? "retry queued" : job.updatedAt,
            }
          : job,
      ),
    );
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

      <div className="stats-row">
        <StatCard label="Running" value="2" detail="workers claimed leases" />
        <StatCard label="Queued" value="1" detail="available now" />
        <StatCard label="Succeeded" value="1" detail="last 10 minutes" />
        <StatCard label="Failed" value="1" detail="needs retry" />
      </div>

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

      <div className="panel">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Job</th>
                <th>Status</th>
                <th>Progress</th>
                <th>Attempts</th>
                <th>Worker</th>
                <th>Updated</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((job) => (
                <tr key={job.id}>
                  <td>
                    <div className="table-title">
                      <strong>{job.type}</strong>
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
                  <td>{job.attempts}</td>
                  <td>{job.worker}</td>
                  <td>{job.updatedAt}</td>
                  <td>
                    <div className="row-actions">
                      <Button onClick={() => updateJob(job.id, "queued")}>Retry</Button>
                      <Button variant="danger" onClick={() => updateJob(job.id, "canceled")}>
                        Cancel
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {visible.length === 0 ? <EmptyState title="No jobs">No queue items match this view.</EmptyState> : null}
      </div>
    </section>
  );
}
