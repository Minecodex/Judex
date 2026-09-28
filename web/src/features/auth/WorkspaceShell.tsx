import { Button, Card } from "@heroui/react";
import { Plus, X } from "lucide-react";
import { AccountSecurity } from "../settings/AccountSecurity";
import { WorkspaceApp } from "../workspace/WorkspaceApp";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";
import { listProjects, logout, type Project } from "./api";
import { useAuth } from "./AuthProvider";
import { errorKey } from "./LoginPage";

// P1 工作区壳（08 §2 首次无项目态）：真实项目列表 + 空态引导 + 账户菜单。
// 聊天工作区整体接入在 P2-08/P6；这里不显示任何演示数据。
export function WorkspaceShell() {
  const { locale, theme, setLocale, setTheme } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const { user, signOut } = useAuth();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [tab, setTab] = useState<"projects" | "security">("projects");
  const [activeProject, setActiveProject] = useState<string | null>(
    () => new URLSearchParams(location.search).get("project"),
  );

  const projects = useQuery({
    queryKey: ["projects"],
    queryFn: listProjects,
  });

  const createProject = useMutation({
    mutationFn: () =>
      request<Project>("/projects", {
        method: "POST",
        body: JSON.stringify({ title: title.trim() }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: () => {
      setTitle("");
      setCreating(false);
      projects.refetch();
    },
  });

  useEffect(() => {
    document.title = "Judex · " + t("workspaceTitle");
  }, [locale]); // eslint-disable-line react-hooks/exhaustive-deps

  const signOutAndRedirect = async () => {
    await logout().catch(() => null);
    await signOut();
    navigate("/login", { replace: true });
  };

  if (activeProject && tab !== "security") {
    return (
      <WorkspaceApp
        projectId={activeProject}
        onLogout={signOutAndRedirect}
        onExit={() => setActiveProject(null)}
      />
    );
  }

  return (
    <div className="judex-workspace-shell">
      <header className="judex-workspace-header">
        <strong>Judex</strong>
        <span className="judex-workspace-user">{t("workspaceWelcome", { name: user?.displayName ?? "" })}</span>
        <div className="judex-workspace-actions">
          <Button variant="ghost" onClick={() => setLocale(locale === "en" ? "zh-CN" : "en")}>
            {locale === "en" ? "中文" : "EN"}
          </Button>
          <Button variant="ghost" onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
            {theme === "dark" ? "☀" : "☾"}
          </Button>
          <Button variant="secondary" onClick={signOutAndRedirect}>
            {t("logout")}
          </Button>
        </div>
      </header>
      <main className="judex-workspace-main">
        <nav className="judex-workspace-tabs" aria-label={t("workspaceTitle")}>
          <Button
            variant={tab === "projects" ? "primary" : "ghost"}
            onClick={() => setTab("projects")}
          >
            {t("workspaceTabProjects")}
          </Button>
          <Button
            variant={tab === "security" ? "primary" : "ghost"}
            onClick={() => setTab("security")}
          >
            {t("workspaceTabSecurity")}
          </Button>
        </nav>
        {tab === "security" ? (
          <AccountSecurity />
        ) : activeProject ? (
          <AccountSecurity />
        ) : projects.isPending ? (
          <p className="judex-workspace-status">{t("shellLoading")}</p>
        ) : projects.isError ? (
          <p className="judex-workspace-status" role="alert">
            {t(errorKey(projects.error) ?? "errNetwork")}
          </p>
        ) : (
          <>
            <div className="judex-workspace-section">
              <h2>{t("workspaceTitle")}</h2>
              <Button data-testid="workspace-new-project" onClick={() => setCreating((v) => !v)}>
                <Plus size={16} />
                {t("workspaceNewProject")}
              </Button>
            </div>
            {creating && (
              <Card className="judex-workspace-create">
                <Card.Content>
                  <form
                    className="judex-auth-form"
                    onSubmit={(event) => {
                      event.preventDefault();
                      if (title.trim()) createProject.mutate();
                    }}
                  >
                    <input
                      className="judex-workspace-input"
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      placeholder={t("workspaceNewProject")}
                      autoFocus
                      aria-label={t("workspaceNewProject")}
                    />
                    <Button type="submit" isPending={createProject.isPending}>
                      {createProject.isPending ? t("submitting") : t("workspaceNewProject")}
                    </Button>
                    <Button variant="ghost" onClick={() => setCreating(false)}>
                      <X size={15} />
                    </Button>
                    {createProject.isError && (
                      <p role="alert">{t(errorKey(createProject.error) ?? "errNetwork")}</p>
                    )}
                  </form>
                </Card.Content>
              </Card>
            )}
            {projects.data && projects.data.length > 0 ? (
              <ul className="judex-workspace-list">
                {projects.data.map((project) => (
                  <li key={project.id} className="judex-workspace-item">
                    <Button variant="secondary" onClick={() => setActiveProject(project.id)}>
                      {project.title}
                    </Button>
                    <small>{project.role ?? ""}</small>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="judex-workspace-empty">
                <strong>{t("workspaceEmpty")}</strong>
                <span>{t("workspaceEmptyHint")}</span>
              </div>
            )}
            {tab === "projects" && (
              <p className="judex-workspace-boundary">{t("workspaceDemoBoundary")}</p>
            )}
          </>
        )}
      </main>
    </div>
  );
}
