import { useEffect } from "react";
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router";
import { Button, Card } from "@heroui/react";
import WorkApp from "../features/chat/ChatWorkspace";
import { usePreferences } from "../stores/preferences";
import { translate, type Key } from "../i18n";
import { dataMode } from "../lib/api/client";
import { AuthProvider, useAuth } from "../features/auth/AuthProvider";
import { LoginPage } from "../features/auth/LoginPage";
import { RegisterPage } from "../features/auth/RegisterPage";
import { RecoverPage } from "../features/auth/RecoverPage";
import { WorkspaceShell } from "../features/auth/WorkspaceShell";

export default function App() {
  const { locale, theme, setLocale, setTheme } = usePreferences();
  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dataset.theme = theme;
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [locale, theme]);
  // 演示模式保留完整旧工作区（tests/e2e fixture）；生产 bundle 走真实认证。
  if (dataMode === "demo") return <WorkApp onLogout={async () => {}} />;
  return (
    <AuthProvider>
      <BrowserRouter>
        <AuthRoutes
          controls={
            <div className="judex-entry-controls">
              <Button variant="ghost" onClick={() => setLocale(locale === "en" ? "zh-CN" : "en")}>
                {locale === "en" ? "中文" : "EN"}
              </Button>
              <Button variant="ghost" onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
                {theme === "dark" ? "☀" : "☾"}
              </Button>
            </div>
          }
        />
      </BrowserRouter>
    </AuthProvider>
  );
}

function AuthRoutes({ controls }: { controls: React.ReactNode }) {
  const { phase, refresh } = useAuth();
  const location = useLocation();
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);

  if (phase === "bootstrapping") {
    return (
      <main className="judex-entry min-h-screen flex items-center justify-center">
        <p className="judex-workspace-status">{t("shellLoading")}</p>
      </main>
    );
  }
  if (phase === "unavailable") {
    return (
      <main className="judex-entry min-h-screen flex flex-col items-center justify-center gap-4 p-8">
        {controls}
        <Card className="judex-auth-card">
          <Card.Header>
            <Card.Title>{t("authEntryTitle")}</Card.Title>
            <Card.Description>{t("errServer")}</Card.Description>
          </Card.Header>
          <Card.Content>
            <Button onClick={() => refresh()}>{t("shellNetwork")}</Button>
          </Card.Content>
        </Card>
      </main>
    );
  }
  const isAuthPage =
    location.pathname === "/login" ||
    location.pathname === "/register" ||
    location.pathname === "/recover";
  return (
    <Routes>
      <Route
        path="/login"
        element={
          phase === "authenticated" ? (
            <Navigate to="/" replace />
          ) : (
            <EntryLayout controls={controls}>
              <LoginPage />
            </EntryLayout>
          )
        }
      />
      <Route
        path="/register"
        element={
          phase === "authenticated" ? (
            <Navigate to="/" replace />
          ) : (
            <EntryLayout controls={controls}>
              <RegisterPage />
            </EntryLayout>
          )
        }
      />
      <Route
        path="/recover"
        element={
          <EntryLayout controls={controls}>
            <RecoverPage />
          </EntryLayout>
        }
      />
      <Route
        path="/*"
        element={
          phase === "authenticated" ? (
            <WorkspaceShell />
          ) : (
            <Navigate
              to="/login"
              replace
              state={{ from: isAuthPage ? "/" : location.pathname + location.search }}
            />
          )
        }
      />
    </Routes>
  );
}

function EntryLayout({
  controls,
  children,
}: {
  controls: React.ReactNode;
  children: React.ReactNode;
}) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  return (
    <main className="judex-entry min-h-screen flex flex-col items-center justify-center gap-6 p-8">
      {controls}
      <Card className="judex-auth-card judex-entry-card">
        <Card.Header>
          <Card.Title>{t("authEntryTitle")}</Card.Title>
          <Card.Description>{t("authEntryHint")}</Card.Description>
        </Card.Header>
        <Card.Content>{children}</Card.Content>
      </Card>
    </main>
  );
}
