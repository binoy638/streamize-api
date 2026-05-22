import { type ReactNode, useEffect, useState } from "react";
import {
  Activity,
  Clapperboard,
  Cog,
  Gauge,
  LinkIcon,
  LogOut,
  Search,
  Users,
} from "lucide-react";
import { NavLink, useLocation } from "react-router-dom";

import { useAuth } from "../lib/auth";
import * as api from "../lib/api";
import { formatBytes } from "../lib/format";
import logoUrl from "../../assets/logo.png";

const pageCopy: Record<string, { title: string; subtitle: string; search: string }> = {
  "/library": {
    title: "Library",
    subtitle: "Playback readiness",
    search: "Search titles, codecs, categories",
  },
  "/torrents": {
    title: "Torrents",
    subtitle: "Transfer records",
    search: "Search torrents, hashes, statuses",
  },
  "/jobs": {
    title: "Jobs",
    subtitle: "Durable processing queue",
    search: "Search jobs, workers, targets",
  },
  "/shares": {
    title: "Shares",
    subtitle: "Expiring public links",
    search: "Search active and expired shares",
  },
  "/admin/users": {
    title: "Admin users",
    subtitle: "Access and quotas",
    search: "Search users",
  },
  "/settings": {
    title: "Settings",
    subtitle: "Server configuration",
    search: "Search settings",
  },
};

function pageFromPath(pathname: string) {
  if (pathname.startsWith("/torrents/")) {
    return {
      title: "Torrent detail",
      subtitle: "Files, jobs, subtitles, shares",
      search: "Search files and jobs",
    };
  }
  if (pathname.startsWith("/player/")) {
    return {
      title: "Player",
      subtitle: "HLS playback surface",
      search: "Search files and subtitles",
    };
  }
  return pageCopy[pathname] || pageCopy["/library"];
}

export function AppShell({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const { user, signOut, apiUnavailable } = useAuth();
  const page = pageFromPath(pathname);
  const [libraryBytes, setLibraryBytes] = useState<number | null>(null);

  // Library size, recomputed on navigation so it reflects added/removed media.
  useEffect(() => {
    let cancelled = false;
    api
      .listAllFiles()
      .then((files) => {
        if (!cancelled) {
          setLibraryBytes(files.reduce((total, file) => total + (file.sizeBytes || 0), 0));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setLibraryBytes(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [pathname]);

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <NavLink className="brand" to="/library">
          <img className="brand-mark" src={logoUrl} alt="" />
          <div className="brand-copy">
            <strong>Streamize</strong>
            <span>media server</span>
          </div>
        </NavLink>

        <nav className="nav-section primary">
          <div className="nav-label">Control</div>
          <NavLink className="nav-item" to="/library">
            <Clapperboard /> Library
          </NavLink>
          <NavLink className="nav-item" to="/torrents">
            <Activity /> Torrents
          </NavLink>
          <NavLink className="nav-item" to="/jobs">
            <Gauge /> Jobs
          </NavLink>
          <NavLink className="nav-item" to="/shares">
            <LinkIcon /> Shares
          </NavLink>
          {user?.role === "admin" ? (
            <NavLink className="nav-item" to="/admin/users">
              <Users /> Admin
            </NavLink>
          ) : null}
        </nav>

        <nav className="nav-section secondary">
          <div className="nav-label">System</div>
          <NavLink className="nav-item" to="/settings">
            <Cog /> Settings
          </NavLink>
        </nav>

        <div className="sidebar-footer">
          <div className="status-line">
            <span>Daemon</span>
            <span>
              <i className={`status-dot ${apiUnavailable ? "offline-dot" : ""}`} />
            </span>
          </div>
          <div className="status-line">
            <span>Storage</span>
            <span className="mono">{libraryBytes === null ? "—" : formatBytes(libraryBytes)}</span>
          </div>
          <div className="status-line">
            <span>{user?.username}</span>
            <button className="text-action" onClick={signOut}>
              <LogOut size={14} /> Sign out
            </button>
          </div>
        </div>
      </aside>

      <main className="main">
        <header className="topbar">
          <div className="page-title">
            <strong>{page.title}</strong>
            <span>{page.subtitle}</span>
          </div>
          <label className="search">
            <Search />
            <input placeholder={page.search} aria-label={page.search} />
          </label>
          <div className="topbar-actions">
            <span className={`badge ${apiUnavailable ? "offline" : "online"}`}>
              {apiUnavailable ? "Prototype" : "Online"}
            </span>
            <span className="badge">{user?.role === "admin" ? "Admin" : "Viewer"}</span>
          </div>
        </header>
        {children}
      </main>
    </div>
  );
}
