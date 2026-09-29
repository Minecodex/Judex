import {
  UIOption,
  UISelect,
  UICheckbox,
  UITextArea,
  UIInput,
} from "../../components/ui/FormControls";
import { useState } from "react";
import { FileText, FolderPlus, Upload as UploadIcon } from "lucide-react";
import { useWork } from "./store";
import { Btn, Dialog, Field, Upload, EvidenceList } from "./ui";
import { type Evidence, type Project } from "./types";
import {
  DEFAULT_DISCUSSION_ROUNDS,
  MAX_DISCUSSION_ROUNDS,
  validRoundLimit,
} from "../chat/discussionPolicy";
export function ExistingAssignments() {
  const { state, project, management, t, text, act } = useWork();
  const [open, setOpen] = useState(false),
    [person, setPerson] = useState(state.currentUser),
    [ids, setIds] = useState<string[]>([]);
  if (!management) return null;
  const available = state.positions.filter(
    (p) =>
      p.projectId === project.id &&
      !state.seats.some((s) => s.person === person && s.positionId === p.id),
  );
  return (
    <>
      <div className="judex-project-assignment">
        <Btn secondary onClick={() => setOpen(true)}>
          {t("projectAssign")}
        </Btn>
      </div>
      {open && (
        <Dialog title={t("projectAssign")} onClose={() => setOpen(false)}>
          <p className="judex-project-note">{t("projectAssignHint")}</p>
          <UISelect
            className="judex-input"
            aria-label={t("workTeam")}
            value={person}
            onChange={(e) => {
              setPerson(e.target.value);
              setIds([]);
            }}
          >
            {project.members.map((m) => (
              <UIOption key={m.name}>{m.name}</UIOption>
            ))}
          </UISelect>
          <div className="judex-position-options">
            {available.map((p) => (
              <UICheckbox
                key={p.id}
                checked={ids.includes(p.id)}
                onChange={(e) =>
                  setIds(
                    e.target.checked
                      ? [...ids, p.id]
                      : ids.filter((id) => id !== p.id),
                  )
                }
              >
                {text(p.name)}
              </UICheckbox>
            ))}
          </div>
          {!available.length && <p>{t("workNoItems")}</p>}
          <Btn
            disabled={!ids.length}
            testId="assign-existing-position"
            onClick={async () => {
              if (
                (
                  await act("assignPositions", {
                    projectId: project.id,
                    person,
                    ids,
                  })
                ).ok
              ) {
                setOpen(false);
                setIds([]);
              }
            }}
          >
            {t("projectAssign")}
          </Btn>
        </Dialog>
      )}
    </>
  );
}
export function ResourcesPage() {
  const { state, project, t, go, act } = useWork();
  const [adding, setAdding] = useState(false),
    [files, setFiles] = useState<Evidence[]>([]),
    [purpose, setPurpose] = useState("");
  const filesById = new Map<string, Evidence>();
  state.tasks
    .filter((v) => v.projectId === project.id)
    .forEach((v) => v.files.forEach((f) => filesById.set(f.id, f)));
  state.handoffs
    .filter((v) => v.projectId === project.id)
    .forEach((v) =>
      v.sources.forEach((s) => s.files.forEach((f) => filesById.set(f.id, f))),
    );
  state.topics
    .filter((v) => v.projectId === project.id)
    .forEach((v) =>
      v.messages.forEach((m) =>
        m.files?.forEach((f) => filesById.set(f.id, f)),
      ),
    );
  return (
    <>
      <div className="judex-project-section-heading">
        <div>
          <small>{t("projectSource")}</small>
          <h2>{t("projectLibrary")}</h2>
        </div>
        <Btn onClick={() => setAdding(true)}>
          <UploadIcon />
          {t("projectRegister")}
        </Btn>
      </div>
      <p className="judex-project-note">{t("projectRegisterHint")}</p>
      {filesById.size ? (
        <EvidenceList files={[...filesById.values()]} />
      ) : (
        <div className="judex-project-empty">
          <FileText />
          <p>{t("projectFilesEmpty")}</p>
        </div>
      )}
      {adding && (
        <Dialog title={t("projectRegister")} onClose={() => setAdding(false)}>
          <Field label={t("projectPurpose")}>
            <UITextArea
              className="judex-input"
              value={purpose}
              onChange={(e) => setPurpose(e.target.value)}
            />
          </Field>
          <Upload files={files} onChange={setFiles} />
          <Btn
            disabled={!files.length}
            onClick={async () => {
              const r = await act("registerResource", {
                projectId: project.id,
                purpose: purpose.trim() || t("projectEvidence"),
                files,
              });
              if (r.ok) {
                setAdding(false);
                setFiles([]);
                setPurpose("");
                go({
                  view: "topic",
                  id: r.id,
                });
              }
            }}
          >
            {t("projectRegister")}
          </Btn>
        </Dialog>
      )}
    </>
  );
}
export function ProjectDialog({ onClose }: { onClose: () => void }) {
  const { t, state, act, go } = useWork();
  const [name, setName] = useState(""),
    [goal, setGoal] = useState(""),
    [maxRounds, setMaxRounds] = useState(String(DEFAULT_DISCUSSION_ROUNDS)),
    [kind, setKind] = useState<Project["kind"]>("software");
  return (
    <Dialog title={t("projectSetupTitle")} onClose={onClose} wide>
      <p className="judex-project-note">{t("projectSetupHint")}</p>
      <Field label={t("projectProjectName")}>
        <UIInput
          className="judex-input"
          data-testid="project-project-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </Field>
      <Field label={t("projectGoal")}>
        <UITextArea
          className="judex-input"
          value={goal}
          onChange={(e) => setGoal(e.target.value)}
        />
      </Field>
      <Field label={t("projectKind")}>
        <UISelect
          className="judex-input"
          value={kind}
          onChange={(e) => setKind(e.target.value as Project["kind"])}
        >
          <UIOption value="software">{t("projectSoftware")}</UIOption>
          <UIOption value="design">{t("projectDesign")}</UIOption>
        </UISelect>
      </Field>
      <Field label={t("chatDiscussionLimit")}>
        <UIInput
          className="judex-input"
          type="number"
          min={1}
          max={MAX_DISCUSSION_ROUNDS}
          step={1}
          value={maxRounds}
          data-testid="new-project-discussion-limit"
          onChange={(e) => setMaxRounds(e.target.value)}
        />
      </Field>
      <p className="judex-project-note">{t("chatRoundMeaning")}</p>
      <Btn
        disabled={
          !name.trim() || !goal.trim() || !validRoundLimit(Number(maxRounds))
        }
        testId="project-create-project"
        onClick={async () => {
          const r = await act("createProject", {
            name,
            goal,
            kind,
            maxRounds: Number(maxRounds),
          });
          if (r.ok && r.id) {
            onClose();
            go({
              projectId: r.id,
              view: "home",
              id: undefined,
            });
          }
        }}
      >
        <FolderPlus />
        {t("projectCreate")}
      </Btn>
    </Dialog>
  );
}
