import { Select, ListBox, Label } from "@heroui/react";
import { FolderOpen, Plus } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { useWork } from "../work/store";
import { member } from "../work/selectors";
import { dataMode } from "../../lib/api/client";
export function ProjectSwitcher({
  onCreate,
  onSwitch,
}: {
  onCreate: () => void;
  onSwitch: () => void;
}) {
  const { state, project, t, text, go } = useWork();
  const projects = state.projects.filter(
    (p) =>
      member(state, p.id) ||
      state.invites.some(
        (i) => i.projectId === p.id && i.person === state.currentUser,
      ),
  );
  return (
    <header className="judex-project-switcher">
      <Button
        isIconOnly
        size="sm"
        className="judex-new-project-top"
        aria-label={t("chatNewProject")}
        onClick={onCreate}
      >
        <Plus size={17} />

      </Button>
      <Select
        aria-label={t("chatSwitchProject")}
        value={project.id}
        onChange={(key) => {
          if (!key) return;
          if (dataMode === "api") {
            location.assign("/?project=" + key);
            return;
          }
          go({
            projectId: String(key),
            view: "home",
            id: undefined,
            conversation: undefined,
          });
          onSwitch();
        }}
        className="judex-project-select-control"
      >
        <Label className="judex-project-select-label">
          {t("chatProjects")}
        </Label>
        <Select.Trigger data-testid="project-switcher">
          <FolderOpen size={18} />
          <span className="judex-project-select-summary"><Select.Value /><small>{t("portalMembers",{count:project.members.length})}</small></span>
          <Select.Indicator />
        </Select.Trigger>
        <Select.Popover className="judex-project-select-popover">
          <ListBox aria-label={t("chatProjects")}>
            {projects.map((p) => (
              <ListBox.Item key={p.id} id={p.id} textValue={text(p.title)}>
                <Label>{text(p.title)}</Label>
                <ListBox.ItemIndicator />
              </ListBox.Item>
            ))}
          </ListBox>
        </Select.Popover>
      </Select>
    </header>
  );
}
