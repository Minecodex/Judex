import { UICard, UIOption, UISelect } from "../../components/ui/FormControls";
import { useState } from "react";
import { Input, Button as HeroButton } from "@heroui/react";
import {
  ArrowLeft,
  Search,
  Settings2,
  SlidersHorizontal,
  Users,
  GitBranch,
  Sparkles,
  Terminal,
  Globe,
  Sun,
  Moon,
  Check,
} from "lucide-react";
import { useWork, WorkView } from "../work/store";
import { Button } from "../../components/ui/Button";
import { PreferencesPage, TeamPage, InvitePage } from "../work/TeamPages";
import { ProjectSettings } from "../chat/ProjectSettings";
import { FlowPage } from "../work/FlowPage";
import type { SettingsSection } from "./navigation";
const sections = [
  {
    id: "general",
    label: "accountGeneral",
    icon: Settings2,
    group: "accountPersonalGroup",
  },
  {
    id: "preferences",
    label: "accountPreferences",
    icon: SlidersHorizontal,
    group: "accountPersonalGroup",
  },
  {
    id: "project",
    label: "accountProjectAI",
    icon: Sparkles,
    group: "accountProjectGroup",
  },
  {
    id: "team",
    label: "accountTeam",
    icon: Users,
    group: "accountProjectGroup",
  },
  {
    id: "flows",
    label: "accountFlows",
    icon: GitBranch,
    group: "accountProjectGroup",
  },
  {
    id: "local",
    label: "accountLocal",
    icon: Terminal,
    group: "accountConnections",
  },
] as const;
export function SettingsScreen() {
  const { route, t, project, text, go, membership, state } = useWork();
  const [search, setSearch] = useState("");
  const active = route.settingsSection ?? "general";
  const visible = sections.filter((section) =>
    t(section.label).toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <main className="judex-settings-page" data-testid="settings-page">
      <aside className="judex-settings-sidebar">
        <Button
          className="judex-settings-back"
          data-testid="settings-back"
          onClick={() =>
            go({
              settingsSection: undefined,
              settingsItem: undefined,
            })
          }
        >
          <ArrowLeft size={18} />
          {t("accountBack")}
        </Button>
        <div className="judex-settings-search">
          <Search size={16} />
          <Input
            aria-label={t("accountSearch")}
            placeholder={t("accountSearch")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <nav aria-label={t("accountSettings")}>
          {(
            [
              "accountPersonalGroup",
              "accountProjectGroup",
              "accountConnections",
            ] as const
          ).map((group) => (
            <section key={group}>
              {visible.some((s) => s.group === group) && (
                <h2>
                  {t(group)}
                  {group === "accountProjectGroup" && (
                    <small>{text(project.title)}</small>
                  )}
                </h2>
              )}
              {visible
                .filter((s) => s.group === group)
                .map((section) => (
                  <Button
                    key={section.id}
                    data-testid={"settings-section-" + section.id}
                    aria-current={active === section.id ? "page" : undefined}
                    className={
                      active === section.id ? "judex-settings-active" : ""
                    }
                    onClick={() =>
                      go({
                        settingsSection: section.id,
                        settingsItem: undefined,
                      })
                    }
                  >
                    <section.icon size={17} />
                    {t(section.label)}
                  </Button>
                ))}
            </section>
          ))}
        </nav>
        {!visible.length && <p>{t("accountNoSettings")}</p>}
      </aside>
      <div className="judex-settings-scroll">
        <div
          className="judex-settings-content"
          key={project.id + state.currentUser + active}
        >
          {!membership && active !== "general" ? (
            <InvitePage />
          ) : (
            <SettingContent section={active} />
          )}
        </div>
      </div>
    </main>
  );
}
function SettingContent({ section }: { section: SettingsSection }) {
  const { t, project, route, state, go } = useWork();
  switch (section) {
    case "general":
      return <GeneralSettings />;
    case "preferences":
      return (
        <WorkView view="settings">
          <PreferencesPage key={project.id + state.currentUser} />
        </WorkView>
      );
    case "project":
      return <ProjectSettings />;
    case "team":
      return (
        <WorkView view="team">
          <TeamPage />
        </WorkView>
      );
    case "flows":
      return (
        <WorkView view="flows" id={route.settingsItem}>
          <FlowPage />
        </WorkView>
      );
    case "local":
      return (
        <section className="judex-settings-local">
          <h1>{t("accountLocal")}</h1>
          <p>{t("chatLocalBody")}</p>
          <UICard className="judex-settings-card">
            <Terminal size={24} />
            <h2>{t("chatLocalTitle")}</h2>
            <p>{t("chatLocalBoundary")}</p>
            <HeroButton
              onPress={() =>
                go({
                  settingsSection: undefined,
                  view: "tasks",
                  id: undefined,
                })
              }
            >
              {t("chatOpenTasks")}
            </HeroButton>
          </UICard>
        </section>
      );
  }
}
function GeneralSettings() {
  const { t, locale, setLocale, theme, setTheme } = useWork();
  return (
    <section>
      <h1>{t("accountGeneral")}</h1>
      <p className="judex-settings-description">{t("accountGeneralHint")}</p>
      <h2 className="judex-settings-section-title">{t("accountAppearance")}</h2>
      <UICard className="judex-settings-card">
        <div className="judex-settings-row">
          <div>
            <Globe size={19} />
            <span>
              <strong>{t("accountLanguage")}</strong>
              <small>{t("accountSaved")}</small>
            </span>
          </div>
          <UISelect
            className="judex-input"
            data-testid="settings-language"
            aria-label={t("accountLanguage")}
            value={locale}
            onChange={(e) => setLocale(e.target.value as typeof locale)}
          >
            <UIOption value="zh-CN">简体中文</UIOption>
            <UIOption value="en">English</UIOption>
          </UISelect>
        </div>
        <div className="judex-settings-row">
          <div>
            {theme === "dark" ? <Moon size={19} /> : <Sun size={19} />}
            <span>
              <strong>{t("accountTheme")}</strong>
              <small>{t("accountSaved")}</small>
            </span>
          </div>
          <div className="judex-settings-theme-options">
            {(["light", "dark"] as const).map((value) => (
              <HeroButton
                key={value}
                variant={theme === value ? "secondary" : "ghost"}
                aria-pressed={theme === value}
                onPress={() => setTheme(value)}
              >
                {theme === value && <Check size={14} />}{" "}
                {t(value === "light" ? "accountLight" : "accountDark")}
              </HeroButton>
            ))}
          </div>
        </div>
      </UICard>
    </section>
  );
}
