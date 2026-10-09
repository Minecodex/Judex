import { Suspense, lazy, useEffect, useState } from "react";
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router";
import { Card } from "@heroui/react";
import { Button } from "../components/ui/Button";
import { AuthStory, Brand, PreferenceControls } from "../components/ui/Presentation";
// Demo workspace is code-split: the production bundle never ships the
// simulated business tree (docs/plans/v1/08 §9).
const DemoPreviewApp = import.meta.env.VITE_DATA_MODE === "demo" ? lazy(() => import("./DemoPreviewApp")) : null;
import { usePreferences } from "../stores/preferences";
import { translate, type Key } from "../i18n";
import { AuthProvider, useAuth } from "../features/auth/AuthProvider";
import { LoginPage } from "../features/auth/LoginPage";
import { RegisterPage } from "../features/auth/RegisterPage";
import { RecoverPage } from "../features/auth/RecoverPage";
import { DevicePage, InvitePage } from "../features/auth/AccessPages";
import { ConfirmPage } from "../features/auth/ConfirmPage";
import { WorkspaceShell } from "../features/auth/WorkspaceShell";

export default function App() {
  const { locale, theme, setLocale, setTheme } = usePreferences();
  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dataset.theme = theme;
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [locale, theme]);
  // 演示模式保留完整旧工作区（tests/e2e fixture）；生产 bundle 走真实认证。
  if (DemoPreviewApp) return <Suspense fallback={<main className="judex-entry" />}><DemoPreviewApp /></Suspense>;
  return (
    <AuthProvider>
      <BrowserRouter>
        <AuthRoutes
          controls={
            <PreferenceControls />
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
            <Button variant="primary" onClick={() => refresh()}>{t("shellNetwork")}</Button>
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
      {[{ path: "/device", element: <DevicePage /> }, { path: "/invite/:token", element: <InvitePage /> }, { path: "/confirm/:intentId", element: <ConfirmPage /> }].map((entry) => (
        <Route key={entry.path} path={entry.path} element={phase === "authenticated"
          ? <EntryLayout controls={controls}>{entry.element}</EntryLayout>
          : <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />} />
      ))}
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
  const location = useLocation();
  const split = ["/login", "/register", "/recover"].includes(location.pathname);
  if (split) return <main className="judex-entry judex-entry-split">
    <div className="judex-entry-controls">{controls}</div>
    <AuthStory /><section className="judex-entry-form-area"><div className="judex-entry-form-wrap">{children}</div><p className="judex-entry-footer">{t("portalFooter")}</p></section>
  </main>;
  return (
    <main className="judex-entry judex-entry-review"><header className="judex-review-header"><Brand small />{controls}</header><div className="judex-entry-review-content">{children}</div></main>
  );
}
