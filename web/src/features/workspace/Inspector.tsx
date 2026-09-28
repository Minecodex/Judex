import { Maximize2, Minimize2, X } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { WorkPanel } from "../work/WorkPanel";
import { MaterialsPanel, TeamPanel } from "../projects/ProjectWorkspace";
import type { PanelView } from "./tabs";

// 右侧功能面板：work（计划/任务/提案/待办，P3-10 真实 API）/
// materials / team；可最大化（占满左栏以外）与收起。
export function Inspector({
  projectId,
  view,
  onView,
  expanded,
  onExpand,
  onClose,
}: {
  projectId: string;
  view: PanelView;
  onView: (view: PanelView) => void;
  expanded: boolean;
  onExpand: () => void;
  onClose: () => void;
}) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const tabs: Array<{ id: PanelView; label: Key }> = [
    { id: "work", label: "wsPanelWork" },
    { id: "materials", label: "wsPanelMaterials" },
    { id: "team", label: "wsPanelTeam" },
  ];
  return (
    <aside className="judex-chat-inspector" data-testid="ws-inspector">
      <header className="judex-inspector-head">
        <nav className="judex-inspector-tabs" aria-label={t("wsPanelWork")}>
          {tabs.map((tab) => (
            <Button
              key={tab.id}
              variant={view === tab.id ? "primary" : "ghost"}
              onClick={() => onView(tab.id)}
              data-testid={"ws-panel-" + tab.id}
            >
              {t(tab.label)}
            </Button>
          ))}
        </nav>
        <div className="judex-inspector-actions">
          <Button
            variant="ghost"
            aria-label={expanded ? t("wsRestore") : t("wsMaximize")}
            title={expanded ? t("wsRestore") : t("wsMaximize")}
            onClick={onExpand}
            data-testid="ws-expand"
          >
            {expanded ? <Minimize2 size={15} /> : <Maximize2 size={15} />}
          </Button>
          <Button
            variant="ghost"
            aria-label={t("wsClosePanel")}
            title={t("wsClosePanel")}
            onClick={onClose}
            data-testid="ws-close-panel"
          >
            <X size={16} />
          </Button>
        </div>
      </header>
      <div className="judex-inspector-body" hidden={false}>
        {view === "work" && <WorkPanel projectId={projectId} />}
        {view === "materials" && <MaterialsPanel projectId={projectId} />}
        {view === "team" && <TeamPanel projectId={projectId} />}
      </div>
    </aside>
  );
}
