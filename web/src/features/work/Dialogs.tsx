import {
  UIInput,
  UITextArea,
  UIOption,
  UISelect,
  UIWarning,
  UICard,
} from "../../components/ui/FormControls";
import { useState } from "react";
import { ArrowRight, Copy } from "lucide-react";
import { useWork } from "./store";
import { Btn, Dialog, EvidenceList, Field, Upload } from "./ui";
import type { Evidence, Task, Handoff, Source, Plan } from "./types";
import {
  createWork,
  reportTask,
  reviseSource,
  sourceDecision,
  taskAction,
  planAction,
} from "./actions";
import { ownsSeat, planTasks, planReviewKey } from "./selectors";
export function CreateWorkDialog({
  kind,
  onClose,
  parentTask,
}: {
  kind: "plan" | "task";
  onClose: () => void;
  parentTask?: Task;
}) {
  const { state, project, route, t, text, act, go } = useWork();
  const seats = state.seats.filter((s) =>
    state.positions.some(
      (p) => p.id === s.positionId && p.projectId === project.id,
    ),
  );
  const flows = state.flows.filter((f) => f.projectId === project.id);
  const [draft, setDraft] = useState({
    kind,
    title: "",
    description: "",
    criteria: "",
    seatId:
      seats.find((s) => s.person === state.currentUser)?.id ??
      seats[0]?.id ??
      "",
    flowId: flows[0]?.id ?? "",
    planId:
      kind === "task" && route.view === "plan"
        ? route.id!
        : (null as string | null),
    parentId: parentTask?.id ?? "",
  });
  const set = (patch: Partial<typeof draft>) =>
    setDraft({
      ...draft,
      ...patch,
    });
  return (
    <Dialog
      title={t(kind === "plan" ? "workNewPlan" : "workNewTask")}
      onClose={onClose}
      wide
    >
      <p className="judex-modal-description">{t("workCreateHint")}</p>
      <Field label={t("workTitle")}>
        <UIInput
          className="judex-input"
          value={draft.title}
          data-testid="new-work-title"
          onChange={(e) =>
            set({
              title: e.target.value,
            })
          }
        />
      </Field>
      <Field label={t("workDescription")}>
        <UITextArea
          className="judex-textarea judex-textarea-short"
          value={draft.description}
          data-testid="new-work-description"
          onChange={(e) =>
            set({
              description: e.target.value,
            })
          }
        />
      </Field>
      <Field label={t("workCriteria")}>
        <UITextArea
          className="judex-textarea judex-textarea-short"
          value={draft.criteria}
          data-testid="new-work-criteria"
          onChange={(e) =>
            set({
              criteria: e.target.value,
            })
          }
        />
      </Field>
      <div className="judex-work-form-grid">
        <Field label={t(kind === "plan" ? "workOwner" : "workChooseSeat")}>
          <UISelect
            className="judex-input"
            value={draft.seatId}
            data-testid="new-work-seat"
            onChange={(e) =>
              set({
                seatId: e.target.value,
              })
            }
          >
            {seats.map((s) => (
              <UIOption key={s.id} value={s.id}>
                {s.person} ·{" "}
                {text(state.positions.find((p) => p.id === s.positionId)!.name)}
              </UIOption>
            ))}
          </UISelect>
        </Field>
        <Field label={t("workChooseFlow")}>
          <UISelect
            className="judex-input"
            value={draft.flowId}
            onChange={(e) =>
              set({
                flowId: e.target.value,
              })
            }
          >
            {flows.map((f) => (
              <UIOption key={f.id} value={f.id}>
                {text(f.name)}
              </UIOption>
            ))}
          </UISelect>
        </Field>
      </div>
      {kind === "task" && (
        <Field label={t("workChoosePlan")}>
          <UISelect
            className="judex-input"
            data-testid="new-work-plan"
            value={draft.planId ?? ""}
            onChange={(e) =>
              set({
                planId: e.target.value || null,
                parentId: "",
              })
            }
          >
            <UIOption value="">{t("workDirect")}</UIOption>
            {state.plans
              .filter((p) => p.projectId === project.id)
              .map((p) => (
                <UIOption key={p.id} value={p.id}>
                  {text(p.title)}
                </UIOption>
              ))}
          </UISelect>
        </Field>
      )}
      {kind === "task" && draft.planId && (
        <Field label={t("cardsParent")}>
          <UISelect
            className="judex-input"
            data-testid="new-work-parent"
            value={draft.parentId}
            onChange={(e) =>
              set({
                parentId: e.target.value,
              })
            }
          >
            <UIOption value="">{t("cardsRootTask")}</UIOption>
            {state.tasks
              .filter(
                (task) =>
                  task.projectId === project.id && task.planId === draft.planId,
              )
              .map((task) => (
                <UIOption key={task.id} value={task.id}>
                  {text(task.title)}
                </UIOption>
              ))}
          </UISelect>
        </Field>
      )}
      {(!seats.length || !flows.length) && (
        <UIWarning className="judex-work-warning">
          {t("cardsNeedInfo")}
        </UIWarning>
      )}
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="create-work-draft"
          disabled={!seats.length || !flows.length}
          onClick={() => {
            let id = "";
            if (
              act((s) => {
                const result = createWork(s, project.id, draft);
                if (result.state)
                  id =
                    kind === "plan"
                      ? result.state.plans.at(-1)!.id
                      : result.state.tasks.at(-1)!.id;
                return result;
              })
            ) {
              onClose();
              go({
                view: kind,
                id,
              });
            }
          }}
        >
          {t("workCreateDraft")}
          <ArrowRight />
        </Btn>
      </div>
    </Dialog>
  );
}
export function ReportDialog({
  task,
  handoff,
  source,
  onClose,
}: {
  task: Task;
  handoff?: Handoff;
  source?: Source;
  onClose: () => void;
}) {
  const { t, text, act } = useWork(),
    [summary, setSummary] = useState(""),
    [files, setFiles] = useState<Evidence[]>([]);
  return (
    <Dialog
      title={t(source ? "workRevise" : "workTaskReport")}
      onClose={onClose}
      wide
    >
      <p className="judex-modal-description">
        {text(task.title)} · {t(source ? "workRevisionHint" : "workLocalHint")}
      </p>
      <Field label={t("workReportLabel")}>
        <UITextArea
          className="judex-textarea"
          data-testid="report-body"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          placeholder={t("workReportPlaceholder")}
        />
      </Field>
      <Upload files={files} onChange={setFiles} />
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="submit-report"
          onClick={() => {
            if (
              act((s) =>
                handoff && source
                  ? reviseSource(
                      s,
                      handoff.id,
                      source.id,
                      source.revision,
                      summary,
                      files,
                    )
                  : reportTask(s, task.id, summary, files),
              )
            )
              onClose();
          }}
        >
          {t(source ? "workSaveRevision" : "workSubmitReport")}
        </Btn>
      </div>
    </Dialog>
  );
}
export function ReasonDialog({
  task,
  handoff,
  source,
  onClose,
}: {
  task?: Task;
  handoff?: Handoff;
  source?: Source;
  onClose: () => void;
}) {
  const { t, act } = useWork(),
    [reason, setReason] = useState("");
  return (
    <Dialog title={t(task ? "workTaskReopen" : "workReject")} onClose={onClose}>
      <Field label={t("workRejectReason")}>
        <UITextArea
          className="judex-textarea"
          data-testid="work-reason"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder={t("workRejectPlaceholder")}
        />
      </Field>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          danger
          testId="confirm-work-reason"
          onClick={() => {
            if (
              act((s) =>
                task
                  ? taskAction(s, task.id, "reopen", reason)
                  : sourceDecision(
                      s,
                      handoff!.id,
                      source!.id,
                      source!.revision,
                      "rejected",
                      reason,
                    ),
              )
            )
              onClose();
          }}
        >
          {t(task ? "workTaskReopen" : "workConfirmReject")}
        </Btn>
      </div>
    </Dialog>
  );
}
export function AcceptanceDialog({
  task,
  plan,
  onClose,
}: {
  task?: Task;
  plan?: Plan;
  onClose: () => void;
}) {
  const { t, text, state, act } = useWork();
  const [review] = useState(() => ({
    tasks: structuredClone(
      plan ? planTasks(state, plan.id) : task ? [task] : [],
    ),
    taskRevision: task?.revision,
    planKey: plan ? planReviewKey(state, plan) : undefined,
  }));
  const tasks = review.tasks;
  const changed = plan
    ? planReviewKey(state, plan) !== review.planKey
    : state.tasks.find((v) => v.id === task?.id)?.revision !==
      review.taskRevision;
  return (
    <Dialog
      title={t(plan ? "workPlanAccept" : "workTaskAccept")}
      onClose={onClose}
      wide
    >
      <p className="judex-modal-description">
        {t(plan ? "workPlanReviewHint" : "workNoReceiptIsAcceptance")}
      </p>
      <UICard className="judex-work-callout">
        <strong>{text(plan?.title ?? task!.title)}</strong>
        <ul>
          {(plan?.criteria ?? task!.criteria).map((c, i) => (
            <li key={i}>{text(c)}</li>
          ))}
        </ul>
      </UICard>
      {tasks.map((item) => (
        <section className="judex-work-review-evidence" key={item.id}>
          <h3>{text(item.title)}</h3>
          <EvidenceList files={item.files} />
        </section>
      ))}
      {changed && (
        <UIWarning className="judex-work-warning" role="alert">
          {t("workErrorStale")}
        </UIWarning>
      )}
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="confirm-final-acceptance"
          disabled={changed}
          onClick={() => {
            if (
              act((s) =>
                plan
                  ? planAction(s, plan.id, "accept", review.planKey)
                  : taskAction(s, task!.id, "accept", "", review.taskRevision),
              )
            )
              onClose();
          }}
        >
          {t(plan ? "workPlanAccept" : "workTaskAccept")}
        </Btn>
      </div>
    </Dialog>
  );
}
export function BriefDialog({
  task,
  onClose,
}: {
  task: Task;
  onClose: () => void;
}) {
  const { state, text, t, setToast } = useWork();
  const flow = state.flows.find((f) => f.id === task.flowId)!;
  const mine = state.preferences.find(
    (p) => p.projectId === task.projectId && p.person === state.currentUser,
  );
  const roles = [...new Set([...task.seatIds, task.reviewerSeatId])]
    .map((id) => state.seats.find((s) => s.id === id)!)
    .filter(Boolean);
  const brief = [
    text(task.title),
    t("workExpected") + ": " + text(task.expected),
    t("workCriteria") +
      ":\n" +
      task.criteria.map((v) => "- " + text(v)).join("\n"),
    t("workBased") + ": " + text(flow.name) + " v" + flow.version,
    text(flow.instructions),
    ...roles.map(
      (seat) =>
        seat.person +
        " · " +
        text(state.positions.find((p) => p.id === seat.positionId)!.prompt),
    ),
    roles.some((s) => ownsSeat(state, s.id))
      ? t("workMyPrompt") + ": " + (mine?.prompt || t("workNoPreference"))
      : t("workPromptPrivate"),
    task.branch ?? "",
    t("workLocalHint"),
    t("workSimulated"),
  ]
    .filter(Boolean)
    .join("\n\n");
  return (
    <Dialog title={t("workBrief")} onClose={onClose} wide>
      <UITextArea
        className="judex-brief-text"
        data-testid="work-brief"
        readOnly
        value={brief}
        rows={16}
      />
      <div className="judex-modal-actions">
        <Btn
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(brief);
              setToast(t("copied"));
            } catch {
              setToast(t("copyFailed"));
            }
          }}
        >
          <Copy />
          {t("workCopyBrief")}
        </Btn>
      </div>
    </Dialog>
  );
}
