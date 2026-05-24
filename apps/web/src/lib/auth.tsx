import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";

import * as api from "./api";

type AuthContextValue = {
  user: api.User | null;
  loading: boolean;
  apiUnavailable: boolean;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<api.User | null>(null);
  const [loading, setLoading] = useState(true);
  const [apiUnavailable, setApiUnavailable] = useState(false);

  useEffect(() => {
    api
      .getMe()
      .then((currentUser) => {
        setUser(currentUser);
        setApiUnavailable(false);
      })
      .catch((error: unknown) => {
        setUser(null);
        setApiUnavailable(!(error instanceof api.ApiError));
      })
      .finally(() => setLoading(false));
  }, []);

  const handleSignIn = useCallback(async (username: string, password: string) => {
    const currentUser = await api.signIn(username, password);
    setUser(currentUser);
    setApiUnavailable(false);
  }, []);

  const handleSignOut = useCallback(async () => {
    try {
      await api.signOut();
    } catch {
      // A stale browser session can still be cleared locally if the API is unreachable.
    }
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      apiUnavailable,
      signIn: handleSignIn,
      signOut: handleSignOut,
    }),
    [apiUnavailable, handleSignIn, handleSignOut, loading, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return context;
}
