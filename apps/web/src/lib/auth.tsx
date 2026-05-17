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
import { demoUser } from "./mock-data";

type AuthContextValue = {
  user: api.User | null;
  loading: boolean;
  apiUnavailable: boolean;
  signIn: (username: string, password: string) => Promise<void>;
  signInPrototype: () => void;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<api.User | null>(null);
  const [loading, setLoading] = useState(true);
  const [apiUnavailable, setApiUnavailable] = useState(false);

  useEffect(() => {
    const localDemo = window.localStorage.getItem("streamize:prototype-user");
    if (localDemo) {
      setUser(demoUser);
      setLoading(false);
      return;
    }

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
    window.localStorage.removeItem("streamize:prototype-user");
    setUser(currentUser);
    setApiUnavailable(false);
  }, []);

  const signInPrototype = useCallback(() => {
    window.localStorage.setItem("streamize:prototype-user", "admin");
    setUser(demoUser);
  }, []);

  const handleSignOut = useCallback(async () => {
    window.localStorage.removeItem("streamize:prototype-user");
    try {
      await api.signOut();
    } catch {
      // Prototype sessions can sign out without a running backend.
    }
    setUser(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      loading,
      apiUnavailable,
      signIn: handleSignIn,
      signInPrototype,
      signOut: handleSignOut,
    }),
    [apiUnavailable, handleSignIn, handleSignOut, loading, signInPrototype, user],
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
