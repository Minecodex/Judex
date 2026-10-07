import { useCollection, LoadMore } from "../../lib/api/collections";
import { Card } from "@heroui/react";
import { Button } from "../../components/ui/Button";
import { FolderPlus, ListChecks, MessageSquare, Plus, UserPlus, X } from "lucide-react";
import { UIStatus } from "../../components/ui/FormControls";
import { AccountSecurity } from "../settings/AccountSecurity";
import ChatWorkspace from "../chat/ChatWorkspace";
import { useMutation } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";
import { logout, type Project } from "./api";
import { useAuth } from "./AuthProvider";
import { errorKey } from "./errors";

// 工作区壳（08 §2 首次无项目态）：真实项目列表 + 空态引导 + 账户菜单。
// 创建成功后直接进入项目工作区；这里不显示任何演示数据。
export function WorkspaceShell() {
  const { locale, theme, setLocale, setTheme } = usePreferences();
  const t = (key: Key, values?: Record<string, string | number>) =>
    translate(locale, key, values);
  const { user, signOut } = useAuth();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [tab, setTab] = useState<"projects" | "security">("projects");
  const route = useLocation();
  const activeProject = new URLSearchParams(route.search).get("project");
  const setActiveProject = (id: string | null) => navigate(id ? `/?project=${id}` : "/");

  const projects = useCollection<Project>(["projects"], "/projects");
  const createProject = useMutation({
    mutationFn: () =>
      request<Project>("/projects", {
        method: "POST",
        body: JSON.stringify({ title: title.trim() }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: (created) => {
      setTitle("");
      setCreating(false);
      projects.refetch();
      setActiveProject(created.id);
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
      <ChatWorkspace
        key={activeProject}
        mode="api"
        projectId={activeProject}
        onLogout={signOutAndRedirect}
      />
    );
  }

  const hasProjects = !!projects.data && projects.data.items.length > 0;

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
              {hasProjects && (
                <Button data-testid="workspace-new-project" onClick={() => setCreating((v) => !v)}>
                  <Plus size={16} />
                  {t("workspaceNewProject")}
                </Button>
              )}
            </div>
            {creating && (
              <Card className="judex-workspace-create">
                <Card.Content>
                  <form
                    className="judex-inline-form"
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
                  </form>
                  {createProject.isError && (
                    <p role="alert">{t(errorKey(createProject.error) ?? "errNetwork")}</p>
                  )}
                </Card.Content>
              </Card>
            )}
            {hasProjects ? (
              <ul className="judex-workspace-list">
                {projects.data?.items.map((project) => (
                  <li key={project.id} className="judex-workspace-item">
                    <Button
                      variant="secondary"
                      className="judex-workspace-item-main"
                      onClick={() => setActiveProject(project.id)}
                    >
                      <span className="judex-workspace-item-title">{project.title}</span>
                    </Button>
                    {project.role && <UIStatus className="judex-workspace-item-role">{project.role}</UIStatus>}
                  </li>
                ))}
              </ul>
            ) : (
              <div className="judex-workspace-empty">
                <span className="judex-workspace-empty-icon" aria-hidden="true">
                  <FolderPlus size={22} />
                </span>
                <strong>{t("workspaceEmpty")}</strong>
                <span>{t("workspaceEmptyHint")}</span>
                <Button data-testid="workspace-new-project" onClick={() => setCreating(true)}>
                  <Plus size={16} />
                  {t("workspaceNewProject")}
                </Button>
                <ul className="judex-workspace-empty-steps">
                  <li>
                    <MessageSquare size={15} aria-hidden="true" />
                    {t("workspaceStepDiscuss")}
                  </li>
                  <li>
                    <ListChecks size={15} aria-hidden="true" />
                    {t("workspaceStepWork")}
                  </li>
                  <li>
                    <UserPlus size={15} aria-hidden="true" />
                    {t("workspaceStepInvite")}
                  </li>
                </ul>
              </div>
            )}
          </>
        )}
        <LoadMore query={projects} />
      </main>
    </div>
  );
}
