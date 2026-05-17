import { type FormEvent, useEffect, useMemo, useState } from "react";

import * as api from "../lib/api";
import { Badge, Button, EmptyState, Field, Input, Modal, Select, StatCard } from "../components/ui";
import { formatBytes, formatDate, toBytes } from "../lib/format";
import { mockUsers } from "../lib/mock-data";

export function AdminUsersPage() {
  const [users, setUsers] = useState<api.User[]>([]);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");
  const [usingMock, setUsingMock] = useState(false);

  useEffect(() => {
    api
      .listUsers()
      .then((result) => {
        setUsers(result);
        setUsingMock(false);
        setError("");
      })
      .catch((err: unknown) => {
        setUsers(mockUsers);
        setUsingMock(true);
        setError(err instanceof Error ? err.message : "Unable to load users");
      });
  }, []);

  const visible = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return users.filter((user) => !normalized || [user.username, user.role].join(" ").toLowerCase().includes(normalized));
  }, [query, users]);

  async function addUser(input: api.CreateUserInput) {
    if (usingMock) {
      const now = new Date().toISOString();
      setUsers((current) => [
        ...current,
        {
          id: `usr_${Date.now()}`,
          username: input.username,
          role: input.role,
          storageQuotaBytes: input.storageQuotaBytes,
          createdAt: now,
          updatedAt: now,
        },
      ]);
      return;
    }

    const user = await api.createUser(input);
    setUsers((current) => [...current, user]);
  }

  return (
    <section className="content">
      <div className="screen-header">
        <div>
          <span className="eyebrow">Admin</span>
          <h1>User access with explicit elevated treatment.</h1>
          <p>Quota, role, status, and destructive actions are surfaced without hiding behind secondary pages.</p>
        </div>
        <div className="header-actions">
          <Button variant="primary" onClick={() => setOpen(true)}>
            Create user
          </Button>
        </div>
      </div>

      <div className="admin-band">
        <strong>Admin-only surface.</strong> Changes here affect authentication, share creation, quotas, and
        reset-password flows.
      </div>

      {error ? (
        <div className="alert alert-warn is-visible">
          Using prototype users because the admin API did not respond: {error}
        </div>
      ) : null}

      <div className="stats-row">
        <StatCard label="Total users" value={String(users.length)} detail={`${users.filter((user) => user.role === "admin").length} admins`} />
        <StatCard label="Active now" value="3" detail="watching or managing" />
        <StatCard label="Disabled" value="1" detail="audit hold" />
        <StatCard label="Storage quota" value="2.4 TB" detail="allocated" />
      </div>

      <label className="inline-search">
        <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search users" />
      </label>

      <div className="panel">
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>User</th>
                <th>Role</th>
                <th>Quota</th>
                <th>Created</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((user) => (
                <tr key={user.id}>
                  <td>
                    <div className="table-title">
                      <strong>{user.username}</strong>
                      <span>{user.id}</span>
                    </div>
                  </td>
                  <td>
                    <Badge tone={user.role === "admin" ? "warn" : "default"}>{user.role}</Badge>
                  </td>
                  <td className="mono">{formatBytes(user.storageQuotaBytes)}</td>
                  <td>{formatDate(user.createdAt)}</td>
                  <td>
                    <Badge tone="online">Active</Badge>
                  </td>
                  <td>
                    <div className="row-actions">
                      <Button disabled>Edit</Button>
                      <Button disabled>Reset</Button>
                      <Button variant="danger" disabled>
                        Disable
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {visible.length === 0 ? <EmptyState title="No users">No users match this query.</EmptyState> : null}
      </div>

      <CreateUserModal open={open} onClose={() => setOpen(false)} onCreate={addUser} />
    </section>
  );
}

function CreateUserModal({
  open,
  onClose,
  onCreate,
}: {
  open: boolean;
  onClose: () => void;
  onCreate: (input: api.CreateUserInput) => Promise<void>;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<api.UserRole>("user");
  const [quota, setQuota] = useState("250 GB");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setMessage("");
    const storageQuotaBytes = toBytes(quota);
    if (!username.trim() || username.trim().length < 3) {
      setMessage("Username must be at least 3 characters.");
      return;
    }
    if (!password.trim() || password.length < 8) {
      setMessage("Password must be at least 8 characters.");
      return;
    }
    if (!Number.isFinite(storageQuotaBytes)) {
      setMessage("Use a quota like 250 GB, 1 TB, or Unlimited.");
      return;
    }

    setBusy(true);
    try {
      await onCreate({ username: username.trim(), password, role, storageQuotaBytes });
      setMessage("User created.");
      setUsername("");
      setPassword("");
      setQuota("250 GB");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to create user");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title="Create user"
      open={open}
      onClose={onClose}
      footer={
        <>
          <Button type="button" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" form="create-user-form" disabled={busy}>
            {busy ? "Creating..." : "Create user"}
          </Button>
        </>
      }
    >
      <form id="create-user-form" className="contents" onSubmit={submit}>
        {message ? <div className={`alert ${message === "User created." ? "alert-success" : "alert-error"} is-visible`}>{message}</div> : null}
        <Field label="Username">
          <Input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="new-user" />
        </Field>
        <Field label="Temporary password">
          <Input value={password} onChange={(event) => setPassword(event.target.value)} type="password" placeholder="minimum 8 characters" />
        </Field>
        <Field label="Role">
          <Select value={role} onChange={(event) => setRole(event.target.value as api.UserRole)}>
            <option value="user">Viewer</option>
            <option value="admin">Admin</option>
          </Select>
        </Field>
        <Field label="Storage quota">
          <Input value={quota} onChange={(event) => setQuota(event.target.value)} placeholder="250 GB" />
        </Field>
      </form>
    </Modal>
  );
}
