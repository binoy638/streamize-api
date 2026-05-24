import { useEffect, useState } from "react";

import * as api from "../lib/api";
import { Badge, EmptyState, Field, Input, StatCard, StatSkeletonGrid } from "../components/ui";

export function SettingsPage() {
  const [health, setHealth] = useState<api.Health | null>(null);
  const [healthError, setHealthError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api
      .getHealth()
      .then((result) => {
        setHealth(result);
        setHealthError("");
      })
      .catch((error: unknown) => {
        setHealth(null);
        setHealthError(error instanceof Error ? error.message : "API unavailable");
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">System settings</span>
          <h1>Server paths, qBittorrent connection, retention, and processing defaults.</h1>
          <p>Runtime health is read from the API. Server configuration is managed by the backend environment.</p>
        </div>
      </div>

      {healthError ? <div className="alert alert-error is-visible">Unable to load API health: {healthError}</div> : null}

      {loading ? (
        <StatSkeletonGrid />
      ) : (
        <div className="stats-row">
          <StatCard label="API health" value={health ? "Online" : "Offline"} detail={health?.status || healthError || "unreachable"} />
          <StatCard label="Database" value={health?.database || "Unknown"} detail="health endpoint" />
          <StatCard label="Service" value={health?.service || "Streamize"} detail="reported by API" />
          <StatCard label="Version" value={health?.version || "Not reported"} detail="build metadata" />
        </div>
      )}

      <div className="layout-grid equal">
        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">API Status</div>
              <p className="muted">Health endpoint response.</p>
            </div>
            <Badge tone={health ? "online" : "offline"}>{health ? "Online" : "Offline"}</Badge>
          </div>
          <div className="form-grid">
            <Field label="Service">
              <Input readOnly value={health?.service || ""} placeholder={loading ? "Loading" : "Unavailable"} />
            </Field>
            <Field label="Database">
              <Input readOnly value={health?.database || ""} placeholder={loading ? "Loading" : "Unavailable"} />
            </Field>
            <Field label="Version">
              <Input readOnly value={health?.version || ""} placeholder={loading ? "Loading" : "Not reported"} />
            </Field>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Configuration</div>
              <p className="muted">Server-managed values.</p>
            </div>
          </div>
          <EmptyState title="No editable settings">
            Configuration values are not exposed by the API yet. This page no longer renders placeholder server paths,
            worker counts, or storage usage.
          </EmptyState>
        </section>
      </div>
    </section>
  );
}
