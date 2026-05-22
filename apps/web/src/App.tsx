import type { ReactNode } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";

import { AuthProvider, useAuth } from "./lib/auth";
import { AppShell } from "./components/AppShell";
import { LoadingScreen } from "./components/ui";
import { AdminUsersPage } from "./pages/AdminUsersPage";
import { JobsPage } from "./pages/JobsPage";
import { LibraryDetailPage } from "./pages/LibraryDetailPage";
import { LibraryPage } from "./pages/LibraryPage";
import { PlayerPage } from "./pages/PlayerPage";
import { SettingsPage } from "./pages/SettingsPage";
import { SharePage } from "./pages/SharePage";
import { SharesPage } from "./pages/SharesPage";
import { SignInPage } from "./pages/SignInPage";
import { TorrentDetailPage } from "./pages/TorrentDetailPage";
import { TorrentsPage } from "./pages/TorrentsPage";
import { WatchPartyPage } from "./pages/WatchPartyPage";

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return <LoadingScreen label="Checking session" />;
  }

  if (!user) {
    return <Navigate to="/sign-in" replace state={{ from: location }} />;
  }

  return <>{children}</>;
}

function RequireAdmin({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  if (user?.role !== "admin") {
    return <Navigate to="/library" replace />;
  }
  return <>{children}</>;
}

function ShellRoute({ children }: { children: ReactNode }) {
  return (
    <RequireAuth>
      <AppShell>{children}</AppShell>
    </RequireAuth>
  );
}

function SignInRoute() {
  const { user } = useAuth();
  const location = useLocation();
  const from = (location.state as { from?: Location } | null)?.from?.pathname || "/library";

  if (user) {
    return <Navigate to={from} replace />;
  }

  return <SignInPage />;
}

function NotFoundRoute() {
  const navigate = useNavigate();
  return (
    <ShellRoute>
      <section className="content">
        <div className="empty-state roomy">
          <strong>Screen not found</strong>
          <p className="muted">The requested Streamize route does not exist in this prototype.</p>
          <button className="btn btn-primary" onClick={() => navigate("/library")}>
            Back to library
          </button>
        </div>
      </section>
    </ShellRoute>
  );
}

export function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/" element={<Navigate to="/library" replace />} />
        <Route path="/sign-in" element={<SignInRoute />} />
        <Route path="/watch/:slug" element={<WatchPartyPage />} />
        <Route path="/s/:slug" element={<SharePage />} />
        <Route
          path="/library"
          element={
            <ShellRoute>
              <LibraryPage />
            </ShellRoute>
          }
        />
        <Route
          path="/library/:id"
          element={
            <ShellRoute>
              <LibraryDetailPage />
            </ShellRoute>
          }
        />
        <Route
          path="/torrents"
          element={
            <ShellRoute>
              <TorrentsPage />
            </ShellRoute>
          }
        />
        <Route
          path="/torrents/:id"
          element={
            <ShellRoute>
              <TorrentDetailPage />
            </ShellRoute>
          }
        />
        <Route
          path="/jobs"
          element={
            <ShellRoute>
              <JobsPage />
            </ShellRoute>
          }
        />
        <Route
          path="/shares"
          element={
            <ShellRoute>
              <SharesPage />
            </ShellRoute>
          }
        />
        <Route
          path="/player/:fileId"
          element={
            <ShellRoute>
              <PlayerPage />
            </ShellRoute>
          }
        />
        <Route
          path="/admin/users"
          element={
            <ShellRoute>
              <RequireAdmin>
                <AdminUsersPage />
              </RequireAdmin>
            </ShellRoute>
          }
        />
        <Route
          path="/settings"
          element={
            <ShellRoute>
              <SettingsPage />
            </ShellRoute>
          }
        />
        <Route path="*" element={<NotFoundRoute />} />
      </Routes>
    </AuthProvider>
  );
}
