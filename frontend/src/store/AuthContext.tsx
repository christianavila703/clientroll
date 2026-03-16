import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from "react";
import api from "../api/client";

interface User {
  id: string;
  tenant_id?: string;
  email: string;
  first_name?: string;
  last_name?: string;
  name?: string;
  role: string;
  setup_completed_at?: string | null;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  isPlatformAdmin: boolean;
  needsSetup: boolean;
  markSetupComplete: () => void;
  login: (email: string, password: string) => Promise<void>;
  platformLogin: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // On mount: verify token
  useEffect(() => {
    const token = localStorage.getItem("access_token");
    if (token) {
      api
        .me()
        .then((data: unknown) => {
          const d = data as { user_id: string; tenant_id: string; role: string; setup_completed_at?: string | null };
          setUser({
            id: d.user_id,
            tenant_id: d.tenant_id,
            email: "",
            role: d.role,
            setup_completed_at: d.setup_completed_at ?? null,
          });
        })
        .catch(() => {
          localStorage.removeItem("access_token");
          localStorage.removeItem("refresh_token");
        })
        .finally(() => setIsLoading(false));
    } else {
      setIsLoading(false);
    }
  }, []);

  // Cross-tab sync: when another tab refreshes the token or logs out,
  // this tab picks it up immediately via the storage event.
  useEffect(() => {
    const handleStorage = (e: StorageEvent) => {
      if (e.key === "access_token") {
        if (!e.newValue) {
          // Token was removed (logged out in another tab)
          setUser(null);
          window.location.href = "/login";
        }
        // Token was updated (refreshed in another tab) — no action needed,
        // the next API call will use the new token from localStorage
      }
    };
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const data = (await api.login(email, password)) as {
      user: User;
      tokens: { access_token: string; refresh_token: string };
    };
    localStorage.setItem("access_token", data.tokens.access_token);
    localStorage.setItem("refresh_token", data.tokens.refresh_token);
    setUser(data.user);
  }, []);

  const platformLogin = useCallback(
    async (email: string, password: string) => {
      const data = (await api.platformLogin(email, password)) as {
        user: User;
        tokens: { access_token: string; refresh_token: string };
      };
      localStorage.setItem("access_token", data.tokens.access_token);
      localStorage.setItem("refresh_token", data.tokens.refresh_token);
      setUser(data.user);
    },
    []
  );

  const markSetupComplete = useCallback(() => {
    setUser((prev) => prev ? { ...prev, setup_completed_at: new Date().toISOString() } : prev);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // Ignore errors on logout
    }
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        isPlatformAdmin: user?.role === "platform_admin",
        needsSetup: !!user && !user.setup_completed_at && user.role !== "platform_admin",
        markSetupComplete,
        login,
        platformLogin,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
