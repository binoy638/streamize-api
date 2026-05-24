import { type FormEvent, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { useAuth } from "../lib/auth";
import { Button, Field, Input } from "../components/ui";
import logoUrl from "../../assets/logo.png";

export function SignInPage() {
  const { signIn, apiUnavailable } = useAuth();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as { from?: Location } | null)?.from?.pathname || "/library";

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    try {
      await signIn(username, password);
      navigate(from, { replace: true });
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Unable to sign in");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-card" aria-labelledby="signin-title">
        <div className="brand" style={{ padding: 0, border: 0 }}>
          <img className="brand-mark" src={logoUrl} alt="" />
          <div className="brand-copy">
            <strong>Streamize</strong>
            <span>Self-hosted media control</span>
          </div>
        </div>
        <h1 id="signin-title">Sign in to your server</h1>
        <p className="muted">
          Use a local admin or viewer account to manage torrents, transcodes, shares, and playback.
        </p>

        <form className="form-grid" onSubmit={submit}>
          {message ? <div className="alert alert-error is-visible">{message}</div> : null}
          {apiUnavailable ? (
            <div className="alert alert-warn is-visible">
              API is not reachable. Start the Streamize API, then sign in with a server account.
            </div>
          ) : null}
          <Field label="Username">
            <Input
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="admin"
            />
          </Field>
          <Field label="Password">
            <Input
              autoComplete="current-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="adminadmin"
            />
          </Field>
          <Button variant="primary" type="submit" disabled={busy}>
            {busy ? "Signing in..." : "Sign in"}
          </Button>
        </form>
      </section>
    </main>
  );
}
