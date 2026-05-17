import type { ReactNode } from "react";
import {
  Activity,
  Clapperboard,
  Cog,
  Gauge,
  LinkIcon,
  LogOut,
  Search,
  Server,
  Users,
} from "lucide-react";
import { NavLink, useLocation } from "react-router-dom";

import { useAuth } from "../lib/auth";
import { mediaItems, torrents } from "../lib/mock-data";

const pageCopy: Record<string, { title: string; subtitle: string; search: string }> = {
  "/library": {
    title: "Library",
    subtitle: `${mediaItems.length * 32} videos indexed`,
    search: "Search titles, codecs, categories",
  },
  "/torrents": {
    title: "Torrents",
    subtitle: `${torrents.length} active records`,
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
  "/style-guide": {
    title: "Style guide",
    subtitle: "Reusable components",
    search: "Search components",
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

  return (
    <div className="app-shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <NavLink className="brand" to="/library">
          <div className="brand-mark">S</div>
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
          <NavLink className="nav-item" to="/style-guide">
            <Server /> Style guide
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
            <span>qBittorrent</span>
            <span className="mono">32 MB/s</span>
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
