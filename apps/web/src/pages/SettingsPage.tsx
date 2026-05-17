import { useEffect, useState } from "react";

import * as api from "../lib/api";
import { Badge, Button, Field, Input, Progress, Select, StatCard } from "../components/ui";
import { settings } from "../lib/mock-data";

export function SettingsPage() {
  const [health, setHealth] = useState<api.Health | null>(null);
  const [healthError, setHealthError] = useState("");

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
      });
  }, []);

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">System settings</span>
          <h1>Server paths, qBittorrent connection, retention, and processing defaults.</h1>
          <p>Mock settings mirror the backend roadmap while health reads the current API if it is running.</p>
        </div>
      </div>

      <div className="stats-row">
        <StatCard label="API health" value={health ? "Online" : "Prototype"} detail={healthError || "reachable"} />
        <StatCard label="qBittorrent" value={settings.qbittorrent} detail="Web API session" />
        <StatCard label="Workers" value={String(settings.workers)} detail="processing slots" />
        <StatCard label="Storage" value={settings.storageUsed} detail={`${settings.storagePercent}% used`} />
      </div>

      <div className="layout-grid equal">
        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Storage</div>
              <p className="muted">Local media roots and database path.</p>
            </div>
            <Badge tone={health ? "online" : "offline"}>{health ? "Online" : "Prototype"}</Badge>
          </div>
          <div className="form-grid">
            <Field label="Media root">
              <Input readOnly value={settings.mediaRoot} />
            </Field>
            <Field label="SQLite path">
              <Input readOnly value={settings.databasePath} />
            </Field>
            <div className="progress-row">
              <Progress value={settings.storagePercent} />
              <span>{settings.storagePercent}%</span>
            </div>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Processing</div>
              <p className="muted">Defaults for HLS, subtitles, thumbnails, and retention.</p>
            </div>
          </div>
          <div className="form-grid">
            <Field label="Retention policy">
              <Select defaultValue="keep">
                <option value="keep">Keep originals after HLS</option>
                <option value="delete_after_hls">Delete originals after HLS</option>
              </Select>
            </Field>
            <Field label="Worker slots">
              <Input defaultValue={settings.workers} type="number" min="1" max="8" />
            </Field>
            <Field label="Subtitle extraction">
              <Select defaultValue="auto">
                <option value="auto">Extract embedded tracks automatically</option>
                <option value="manual">Manual only</option>
              </Select>
            </Field>
            <Button variant="primary">Save mock settings</Button>
          </div>
        </section>
      </div>
    </section>
  );
}
