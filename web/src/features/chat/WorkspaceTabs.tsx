import { useEffect, useRef } from "react";
import { Tabs } from "@heroui/react";
import {
  GitBranch,
  Files,
  Inbox,
  Check,
  ArrowUpRight,
  Plus,
  X,
  Maximize2,
  Minimize2,
  LayoutGrid,
} from "lucide-react";
import { Button } from "../../components/ui/Button";
import { useWork, WorkView } from "../work/store";
import { WorkspacePanel } from "./WorkspacePanel";
import { useWorkspaceTabs } from "./useWorkspaceTabs";
import { workspaceKey, type WorkspaceTab } from "./workspaceTabModel";
import { useReadingPosition } from "./useReadingPosition";
const tools = [
  { view: "plans", label: "chatRoadmap", icon: GitBranch, testId: "plans" },
  {
    view: "resources",
    label: "chatMaterials",
    icon: Files,
    testId: "resources",
  },
  {
    view: "decisions",
    label: "chatDecisions",
    icon: Check,
    testId: "decisions",
  },
  { view: "handoffs", label: "workHandoffs", icon: Inbox, testId: "handoffs" },
  {
    view: "overview",
    label: "chatOverview",
    icon: LayoutGrid,
    testId: "overview",
  },
] as const;
export function WorkspaceTabs({
  expanded,
  onExpand,
  onClose,
}: {
  expanded: boolean;
  onExpand: () => void;
  onClose: () => void;
}) {
  const { state, project, text, t } = useWork(),
    model = useWorkspaceTabs(),
    bar = useRef<HTMLDivElement>(null);
  const info = (tab: WorkspaceTab) => {
    const tool = tools.find((v) => workspaceKey(v) === workspaceKey(tab));
    return {
      icon: tool?.icon ?? GitBranch,
      title: tool
        ? t(tool.label)
        : text(
            (tab.view === "task" ? state.tasks : state.plans).find(
              (v) => v.id === tab.id && v.projectId === project.id,
            )?.title ??
              tab.id ??
              "",
          ),
    };
  };
  useEffect(() => {
    bar.current
      ?.querySelector('[aria-selected="true"]')
      ?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [model.active]);
  return (
    <>
      <Tabs
        className="judex-workspace-root"
        selectedKey={model.active ?? "launcher"}
        onSelectionChange={(key) => {
          if (key === "launcher") model.launcher();
          else {
            const tab = model.tabs.find((t) => workspaceKey(t) === key);
            if (tab) model.open(tab);
          }
        }}
      >
        <div
          className="judex-conversation-tabstrip judex-workspace-tabstrip"
          ref={bar}
        >
          <Tabs.List
            className="judex-conversation-tabs"
            aria-label={t("chatWorkspaceTabs")}
          >
            {model.tabs.map((tab) => {
              const key = workspaceKey(tab),
                { icon: Icon, title } = info(tab);
              return (
                <Tabs.Tab
                  key={key}
                  id={key}
                  aria-label={title}
                  data-testid={"workspace-tab-" + key}
                  className={
                    "judex-conversation-tab" +
                    (model.active === key
                      ? " judex-conversation-tab-active"
                      : "")
                  }
                >
                  <Icon size={14} />
                  <span className="judex-conversation-tab-label" title={title}>
                    {title}
                  </span>
                  <Button
                    className="judex-conversation-tab-close"
                    aria-label={t("chatCloseTool") + " · " + title}
                    onPointerDown={(e) => e.stopPropagation()}
                    onClick={(e) => {
                      e.stopPropagation();
                      model.close(key);
                    }}
                  >
                    <X size={13} />
                  </Button>
                </Tabs.Tab>
              );
            })}
            {!model.active && (
              <Tabs.Tab
                id="launcher"
                className="judex-conversation-tab judex-conversation-tab-active"
                aria-label={t("chatChooseTool")}
              >
                <LayoutGrid size={14} />
                <span className="judex-conversation-tab-label">
                  {t("chatChooseTool")}
                </span>
              </Tabs.Tab>
            )}
          </Tabs.List>
          <Button
            data-testid="workspace-add-tool"
            aria-label={t("chatAddTool")}
            onClick={model.launcher}
          >
            <Plus size={16} />
          </Button>
          <div className="judex-workspace-window-controls">
            <Button
              data-testid="workspace-maximize"
              aria-label={t(
                expanded ? "chatRestoreWorkspace" : "chatMaxWorkspace",
              )}
              onClick={onExpand}
            >
              {expanded ? <Minimize2 size={15} /> : <Maximize2 size={15} />}
            </Button>
            <Button aria-label={t("chatCollapse")} onClick={onClose}>
              <X size={17} />
            </Button>
          </div>
        </div>
        <Tabs.Panel
          id={model.active ?? "launcher"}
          className="judex-workspace-tab-content"
        >
          {model.current ? (
            <WorkView view={model.current.view} id={model.current.id}>
              <WorkspaceContent
                key={model.scope + ":" + model.active}
                tabKey={model.active!}
              />
            </WorkView>
          ) : (
            <div
              className="judex-workspace-launcher"
              data-testid="workspace-launcher"
            >
              <div>
                <h2>{t("chatChooseTool")}</h2>
                <p>{t("chatChooseToolHint")}</p>
                <div className="judex-workspace-launcher-list">
                  {tools.map((tool) => (
                    <Button
                      key={tool.testId}
                      data-testid={"work-nav-" + tool.testId}
                      onClick={() => model.open(tool)}
                    >
                      <tool.icon size={18} />
                      <span>{t(tool.label)}</span>
                      <ArrowUpRight size={14} />
                    </Button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </Tabs.Panel>
      </Tabs>
    </>
  );
}
function WorkspaceContent({ tabKey }: { tabKey: string }) {
  const { project, state } = useWork();
  const ref = useReadingPosition(
    "workspace:" + project.id + ":" + state.currentUser + ":" + tabKey,
  );
  return (
    <div className="judex-chat-panel-body judex-next-main" ref={ref}>
      <WorkspacePanel />
    </div>
  );
}
