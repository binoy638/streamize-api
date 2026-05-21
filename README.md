# Streamize

Streamize is being rewritten as a Go-first monorepo for self-hosted torrent-backed video streaming.

The current TypeScript API has been preserved under `legacy/api` while the new implementation is built in `apps/api`.

## Repository Layout

```text
apps/
  api/   Go API, workers, SQLite schema, and future static UI serving
  web/   Future React + Vite UI
deploy/ Docker Compose and environment examples
docs/   Architecture and implementation notes
legacy/ Previous TypeScript API kept for reference during the rewrite
```

## Backend Development

```bash
make api-test
make api-dev
```

The API defaults to `http://localhost:8080` and exposes:

```text
GET /api/health
POST /api/auth/sign-in
POST /api/auth/sign-out
GET /api/auth/me
GET /api/torrents
POST /api/torrents
GET /api/torrents/{id}/files
DELETE /api/torrents/{id}
GET /api/jobs
POST /api/jobs/{id}/retry
POST /api/jobs/{id}/cancel
GET /api/files/{id}/original
GET /api/files/{id}/hls/index.m3u8
GET /api/files/{id}/hls/{segment}
GET /api/files/{id}/subtitles
GET /api/files/{id}/preview/thumbnails.vtt
GET /api/files/{id}/preview/{asset}
GET /api/subtitles/{id}/track.vtt
GET /api/admin/users
POST /api/admin/users
```

In development, the bootstrap admin defaults to `admin` / `adminadmin`. Override it with `STREAMIZE_ADMIN_USERNAME` and `STREAMIZE_ADMIN_PASSWORD`.

Media workers use `ffmpeg` and `ffprobe` by default. Override them with `STREAMIZE_FFMPEG_PATH` and `STREAMIZE_FFPROBE_PATH` if the binaries live outside `PATH`.

## Web Development

The React + Vite UI lives in `apps/web` and implements the design-spec prototype as production React screens.

```bash
cd apps/web
npm install
npm run dev
```

The web dev server defaults to `http://localhost:5173` and proxies `/api` to the Go API at `http://localhost:8080`. If the API is not running, use the sign-in screen's prototype entry to browse the mock media flows.

## Docker Compose

Copy the example environment file and adjust values if needed:

```bash
cp deploy/.env.example deploy/.env
mkdir -p data/qbittorrent-config media/originals media/hls media/subtitles media/thumbnails media/tmp
chmod -R u+rwX,g+rwX data media
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
```

The Compose stack runs the Go API and qBittorrent. SQLite, qBittorrent config, and media files are bind-mounted into local `data/` and `media/` folders so downloads are visible on the host.

## Production Deployment

Production deploys are continuous: every push to `master` (and the manual **Run workflow** button) triggers `.github/workflows/deploy.yml`, which builds a single Docker image — the Go API with the React UI embedded — pushes it to GitHub Container Registry, then SSHes to the VPS to pull and restart.

```text
Internet ──443──> Traefik (TLS) ──> api:8080 (Go + embedded SPA) ──> qbittorrent
```

TLS and routing are handled by an existing Traefik instance on the VPS. The `api` container
joins Traefik's Docker network and is routed by labels in `docker-compose.prod.yml`
(`Host(${DOMAIN})` → port `8080`, cert resolver `mytlschallenge`). Set `TRAEFIK_NETWORK` in
`.env` to the network Traefik watches.

### One-time VPS setup

1. **Install Docker** (Engine + compose plugin) and add the deploy user to the `docker` group:
   ```bash
   curl -fsSL https://get.docker.com | sh
   sudo usermod -aG docker "$USER"   # re-login afterwards
   ```
2. **Create the deploy directory** and runtime folders. The `api` and
   `qbittorrent` containers both run as the uid/gid given by `PUID`/`PGID` in
   `.env` — set those to the **deploy user's own ids** so bind-mounted files
   line up (on many cloud images the default user is `1001`, not `1000`):
   ```bash
   id   # note your uid/gid — use them for PUID/PGID in .env (step 3)

   sudo mkdir -p /opt/streamize && sudo chown "$USER" /opt/streamize
   mkdir -p /opt/streamize/data/qbittorrent-config \
            /opt/streamize/media/{originals,hls,subtitles,thumbnails,tmp}
   sudo chown -R "$(id -u):$(id -g)" /opt/streamize/data /opt/streamize/media
   ```
3. **Add the environment file** — copy `deploy/.env.prod.example` to `/opt/streamize/.env` and set real secrets plus `DOMAIN`.
4. **SSH key** — add the deploy public key to the VPS user's `~/.ssh/authorized_keys`.
5. **DNS** — point an `A` record for `DOMAIN` at the VPS public IP.
6. **Firewall** — allow `22` and `6881` (tcp+udp). Ports `80`/`443` are already served by Traefik.

### GitHub configuration

Add these repository **secrets** (Settings → Secrets and variables → Actions):

| Secret | Value |
|---|---|
| `VPS_HOST` | VPS hostname or IP |
| `VPS_USER` | deploy SSH user |
| `VPS_SSH_KEY` | the deploy **private** key |
| `VPS_PORT` | SSH port (optional, defaults to `22`) |

GHCR push uses the built-in `GITHUB_TOKEN` — no extra secret needed. After the first deploy publishes the package, set its visibility to **public** under the repo's Packages settings so the VPS can pull without registry auth. (No secrets are baked into the image — all config comes from `.env` at runtime.)

The deploy job copies `docker-compose.prod.yml` to `/opt/streamize` on each run, then runs `docker compose pull && up -d`. Database migrations apply automatically on API start.

### qBittorrent WebUI

The WebUI port is bound to `127.0.0.1` on the VPS — reach it through an SSH tunnel rather than the public internet. Point ssh at your VPS private key with `-i`:

```bash
chmod 600 /path/to/vps-key                                  # one-time: ssh rejects loose perms
ssh -i /path/to/vps-key -L 8081:127.0.0.1:8081 ubuntu@<VPS_HOST>
```

While that session is open, browse `http://localhost:8081` for the qBittorrent WebUI.

To avoid passing `-i` every time, add an entry to `~/.ssh/config`:

```
Host streamize-vps
    HostName <VPS_HOST>
    User ubuntu
    IdentityFile /path/to/vps-key
```

Then the tunnel shortens to `ssh -L 8081:127.0.0.1:8081 streamize-vps`.

qBittorrent 5.x ships **no default password**. On a fresh config it prints a one-time
WebUI password in the container logs (`docker logs streamize-qbittorrent-1 | grep -i password`).
Log in with it, then set the WebUI password (Tools → Options → Web UI) to match
`STREAMIZE_QBITTORRENT_PASSWORD` in `.env` — otherwise the API gets `502 Bad Gateway`
on every torrent operation.

### Troubleshooting

**Ownership rules — most deploy issues trace back to these.** There are two distinct
owners under `/opt/streamize`, and they must not be collapsed:

| Path | Owner | Why |
|---|---|---|
| `/opt/streamize/` and the files in it (`.env`, `docker-compose.prod.yml`) | the **deploy SSH user** | CI must overwrite the compose file on every deploy |
| `/opt/streamize/data/` and `/opt/streamize/media/` | **`PUID:PGID`** | the `api` and `qbittorrent` containers read/write them |

Never run `chown -R` on `/opt/streamize` as a whole — it forces one owner onto both
and breaks either CI or the containers. If `PUID`/`PGID` equal the deploy user's own
ids (the recommended setup), the two happen to coincide, but still chown the subtrees
separately so the intent is explicit.

**CI deploy fails with `tar: docker-compose.prod.yml: Cannot open: Permission denied`.**
The deploy user can't overwrite the compose file — it or `/opt/streamize` is owned by
`root`, usually left over from earlier `sudo` use. Fix on the VPS, then re-run the workflow:

```bash
sudo chown <deploy-user>:<deploy-user> /opt/streamize
sudo rm -f /opt/streamize/docker-compose.prod.yml   # drop the stale root-owned file
```

**qBittorrent WebUI unreachable** — `curl http://127.0.0.1:8081` returns
`Connection reset by peer`, and the API logs show
`dial tcp ...:8081: connect: no route to host`. qBittorrent is crash-looping *inside*
its container. `docker compose ps` can still show it `Up` — that is the container's
init process (`/init`), not qBittorrent itself. Confirm by checking the logs:

```bash
docker logs --tail=100 streamize-qbittorrent-1
```

If the LinuxServer banner prints more than once and the logs never reach
`WebUI: Now listening`, the qBittorrent process keeps dying. The usual cause is a
config it cannot read — check that `data/qbittorrent-config` is owned by `PUID:PGID`,
or reset the config (below).

**Reset qBittorrent config** — archives the settings file so a fresh one is generated
on next start. Downloaded torrents (`BT_backup/`) are untouched:

```bash
docker compose -f docker-compose.prod.yml stop qbittorrent
sudo mv data/qbittorrent-config/qBittorrent/qBittorrent.conf{,.bak}
docker compose -f docker-compose.prod.yml up -d qbittorrent
```

**Full reset** — tear down and wipe all state (DB, qBittorrent config, all media):

```bash
docker compose -f docker-compose.prod.yml down --remove-orphans
sudo rm -rf data media
mkdir -p data media && sudo chown <PUID>:<PGID> data media
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

**Container-to-container `no route to host` that persists after the above** usually
means Docker's iptables rules were flushed (often by a `ufw`/`firewalld` reload).
Rebuild them with `sudo systemctl restart docker`, then bring the stack back up.
