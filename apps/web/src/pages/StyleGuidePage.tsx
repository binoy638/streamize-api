import { Badge, Button, EmptyState, Field, Input, Progress, Select, StatCard, Textarea } from "../components/ui";

export function StyleGuidePage() {
  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Components</span>
          <h1>Reusable Streamize components extracted from the design handoff.</h1>
          <p>These primitives keep the prototype visually consistent while backend areas are wired in phases.</p>
        </div>
      </div>

      <div className="stats-row">
        <StatCard label="Radius" value="10px" detail="cards and panels" />
        <StatCard label="Accent" value="Cyan" detail="primary actions" />
        <StatCard label="Motion" value="160ms" detail="hover and state" />
        <StatCard label="Theme" value="Dark" detail="operational media UI" />
      </div>

      <div className="layout-grid equal">
        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Buttons and badges</div>
              <p className="muted">Primary, ghost, danger, and status treatments.</p>
            </div>
          </div>
          <div className="component-stack">
            <div className="component-row">
              <Button variant="primary">Primary</Button>
              <Button>Ghost</Button>
              <Button variant="danger">Danger</Button>
              <Button disabled>Disabled</Button>
            </div>
            <div className="component-row">
              <Badge tone="ready">Ready</Badge>
              <Badge tone="processing">Processing</Badge>
              <Badge tone="failed">Failed</Badge>
              <Badge tone="online">Online</Badge>
              <Badge tone="warn">Warn</Badge>
            </div>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Forms</div>
              <p className="muted">Fields match the modal and settings layouts.</p>
            </div>
          </div>
          <div className="form-grid">
            <Field label="Input">
              <Input placeholder="magnet:?xt=urn:btih:..." />
            </Field>
            <Field label="Select">
              <Select defaultValue="ready">
                <option value="ready">Ready</option>
                <option value="processing">Processing</option>
              </Select>
            </Field>
            <Field label="Textarea">
              <Textarea placeholder="Paste a magnet URL" />
            </Field>
          </div>
        </section>
      </div>

      <div className="layout-grid equal">
        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">Progress</div>
              <p className="muted">Used across torrents, jobs, player health, and storage.</p>
            </div>
          </div>
          <div className="component-stack">
            <div className="progress-row">
              <Progress value={100} />
              <span>Ready</span>
            </div>
            <div className="progress-row">
              <Progress value={62} />
              <span>62%</span>
            </div>
            <div className="progress-row">
              <Progress value={18} />
              <span>18%</span>
            </div>
          </div>
        </section>

        <section className="panel">
          <div className="panel-header">
            <div>
              <div className="panel-title">States</div>
              <p className="muted">Loading, empty, alert, and success/error states are first-class.</p>
            </div>
          </div>
          <div className="component-stack">
            <EmptyState title="No media in filter">Search and filters collapse to this state.</EmptyState>
            <div className="skeleton-grid">
              <div className="skeleton" />
              <div className="skeleton" />
            </div>
            <div className="alert alert-success is-visible">Share created.</div>
            <div className="alert alert-error is-visible">Thumbnail extraction failed.</div>
          </div>
        </section>
      </div>
    </section>
  );
}
