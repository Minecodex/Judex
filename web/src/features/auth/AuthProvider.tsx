import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchSession, type PublicUser } from "./api";
import { APIError } from "../../lib/api/client";

// 认证状态机（docs/plans/v1/08 §2）：bootstrapping → anonymous /
// authenticated → signingOut。GET session 区分 401（未登录）与服务不可用。
export type AuthPhase = "bootstrapping" | "anonymous" | "authenticated" | "unavailable";

type AuthState = {
  phase: AuthPhase;
  user: PublicUser | null;
  refresh: () => Promise<void>;
  onAuthenticated: (user: PublicUser) => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const client = useQueryClient();
  const [phase, setPhase] = useState<AuthPhase>("bootstrapping");
  const [user, setUser] = useState<PublicUser | null>(null);

  const refresh = useCallback(async () => {
    try {
      const session = await fetchSession();
      setUser(session.user);
      setPhase("authenticated");
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        setUser(null);
        setPhase("anonymous");
      } else {
        // 5xx/网络故障：明确显示不可用，不伪造登录态（08 §2）。
        setUser(null);
        setPhase("unavailable");
      }
    }
  }, []);

  const onAuthenticated = useCallback(
    async (nextUser: PublicUser) => {
      setUser(nextUser);
      setPhase("authenticated");
      // 注册/登录建立会话后立即拉取 session（获取 CSRF token）并刷新缓存。
      await refresh();
      await client.invalidateQueries();
    },
    [client, refresh],
  );

  const signOut = useCallback(async () => {
    setPhase("bootstrapping");
    await client.cancelQueries();
    client.clear();
    await fetchSession().catch(() => null); // best-effort; 401 clears nothing
    setUser(null);
    setPhase("anonymous");
  }, [client]);

  const sessionQuery = useQuery({
    queryKey: ["auth", "session"],
    queryFn: refresh,
    staleTime: 60_000,
    retry: false,
    enabled: phase === "bootstrapping",
  });

  const value = useMemo<AuthState>(
    () => ({ phase, user, refresh, onAuthenticated, signOut }),
    [phase, user, refresh, onAuthenticated, signOut],
  );
  void sessionQuery;
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider required");
  return value;
}

// returnTo 仅接受本站相对路径，拒绝协议与双斜杠跳转（08 §2）。
export function safeReturnTo(candidate: string | null | undefined): string {
  if (!candidate) return "/";
  if (!candidate.startsWith("/") || candidate.startsWith("//")) return "/";
  return candidate;
}
