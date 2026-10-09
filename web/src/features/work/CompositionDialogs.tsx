import {
  UIOption,
  UISelect,
  UICheckbox,
  UIInput,
} from "../../components/ui/FormControls";
import { useState } from "react";
import { useWork } from "./store";
import { dataMode } from "../../lib/api/client";
import { Btn, Dialog, Field } from "./ui";
import type { Task, Handoff } from "./types";
import {ResponsibilityPicker} from "./ResponsibilityPicker";
import {handoffResponsibilities,defaultHandoffSender} from "./handoffResponsibilities";
export function HandoffDialog({
  task,
  onClose,
}: {
  task: Task;
  onClose: () => void;
}) {
  const { state, project, t, text, act, go } = useWork();
  const [senders,setSenders]=useState<Record<string,string>>({}),[busy,setBusy]=useState(false);
  const [kind, setKind] = useState<Handoff["kind"]>("stage"),
    [target, setTarget] = useState(task.id),
    [receiver, setReceiver] = useState(task.reviewerSeatId),
    [ids, setIds] = useState([task.id]);
  const otherTasks = state.tasks.filter(
    (v) =>
      v.projectId === project.id &&
      v.id !== task.id &&
      v.status !== "draft" &&
      (!v.planId ||
        state.plans.find((p) => p.id === v.planId)?.status !== "draft"),
  );
  const next = state.tasks.find((t) => t.id === target)!;
  const recipients = [...new Set([...next.seatIds, next.reviewerSeatId])];
  const eligible = state.tasks.filter(
    (v) =>
      v.projectId === project.id &&
      ["delivered", "accepted"].includes(v.status) &&
      (v.files.length > 0 || dataMode === "api") &&
      (kind === "stage" ? v.id === task.id : v.id !== target),
  );
  const responsibilities=handoffResponsibilities(state,project.id),selected=ids.map(id=>state.tasks.find(v=>v.id===id)).filter((v):v is Task=>!!v);
  const senderIds=Object.fromEntries(selected.map(item=>[item.id,senders[item.id]??defaultHandoffSender(state,item)]));
  const valid=!!selected.length&&selected.length===ids.length&&selected.every(item=>eligible.some(v=>v.id===item.id)&&responsibilities.includes(senderIds[item.id]))&&responsibilities.includes(receiver);
  return (
    <Dialog title={t("workProposeHandoff")} onClose={onClose} wide>
      <p className="judex-modal-description">{t("workProposeHandoffHint")}</p>
      <Field label={t("workHandoffType")}>
        <UISelect
          className="judex-input"
          data-testid="handoff-kind"
          value={kind}
          onChange={(e) => {
            const k = e.target.value as Handoff["kind"];
            setKind(k);
            if (k === "stage") {
              setTarget(task.id);
              setReceiver(task.reviewerSeatId);
              setIds([task.id]);
            } else {
              const v = otherTasks[0];
              if (!v) return;
              setTarget(v.id);
              setReceiver(v.seatIds[0]);
              setIds([task.id]);
            }
          }}
        >
          <UIOption value="stage">{t("workHandoffKindStage")}</UIOption>
          <UIOption value="dependency" disabled={!otherTasks.length}>
            {t("workHandoffKindDependency")}
          </UIOption>
        </UISelect>
      </Field>
      {kind === "dependency" && (
        <Field label={t("workNextTask")}>
          <UISelect
            className="judex-input"
            data-testid="handoff-target"
            value={target}
            onChange={(e) => {
              const v = state.tasks.find((v) => v.id === e.target.value)!;
              setTarget(v.id);
              setReceiver(v.seatIds[0]);
              setIds(ids.filter((id) => id !== v.id));
            }}
          >
            {otherTasks.map((v) => (
              <UIOption key={v.id} value={v.id}>
                {text(v.title)}
              </UIOption>
            ))}
          </UISelect>
        </Field>
      )}
      <Field label={t("workReceiver")}>
        <UISelect
          className="judex-input"
          data-testid="handoff-receiver"
          value={receiver}
          onChange={(e) => setReceiver(e.target.value)}
        >
          {recipients.map((id) => (
            <UIOption key={id} value={id}>
              {state.seats.find((s) => s.id === id)?.person}
            </UIOption>
          ))}
        </UISelect>
      </Field>
      <div className="judex-position-options">
        {eligible.map((v) => (
          <UICheckbox appearance="card"
            key={v.id}
            data-testid={"handoff-source-" + v.id}
            checked={ids.includes(v.id)}
            onChange={(e) =>
              setIds(
                e.target.checked
                  ? [...ids, v.id]
                  : ids.filter((id) => id !== v.id),
              )
            }
          >
            <span>{text(v.title)}</span>
          </UICheckbox>
        ))}
      </div>
      {selected.map(item=><Field key={item.id} label={t("coHandoffSenderFor",{task:text(item.title)})}><ResponsibilityPicker ids={responsibilities} value={senderIds[item.id]} onChange={id=>setSenders(v=>({...v,[item.id]:id}))} testId={"handoff-sender-"+item.id} label={t("coChooseHandoffSender")} showPerson/></Field>)}
      <p className="judex-modal-description">{t("coHandoffSenderHint")}</p>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="create-handoff-draft"
          disabled={!valid||busy}
          onClick={async () => {
            if(!valid||busy)return;setBusy(true);
            try{
            const r = await act("proposeHandoff", {
              projectId: project.id,
              target,
              receiver,
              ids,
              kind,
              senderIdentityIds:senderIds,
            });
            if (r.ok) {
              onClose();
              go({
                page:"hub",hubTab:"deliveries",scopePlanId:undefined,scopeTaskId:undefined,
                view: "handoff",
                id: r.id,
              });
            }
            }finally{setBusy(false);}
          }}
        >
          {t("workCreateDraft")}
        </Btn>
      </div>
    </Dialog>
  );
}
export function DiscussionDialog({ onClose }: { onClose: () => void }) {
  const { state, project, t, text, act, go } = useWork(),
    [title, setTitle] = useState(""),
    [planIds, setPlans] = useState<string[]>([]),
    [taskIds, setTasks] = useState<string[]>([]);
  return (
    <Dialog title={t("workNewDiscussion")} onClose={onClose} wide>
      <p className="judex-modal-description">{t("workDiscussionLinkHint")}</p>
      <Field label={t("workTitle")}>
        <UIInput
          className="judex-input"
          data-testid="discussion-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </Field>
      <h3>{t("workPlans")}</h3>
      <div className="judex-position-options">
        {state.plans
          .filter((p) => p.projectId === project.id)
          .map((p) => (
            <UICheckbox appearance="card"
              key={p.id}
              checked={planIds.includes(p.id)}
              data-testid={"topic-plan-" + p.id}
              onChange={(e) =>
                setPlans(
                  e.target.checked
                    ? [...planIds, p.id]
                    : planIds.filter((id) => id !== p.id),
                )
              }
            >
              {text(p.title)}
            </UICheckbox>
          ))}
      </div>
      <h3>{t("workTasks")}</h3>
      <div className="judex-position-options">
        {state.tasks
          .filter((p) => p.projectId === project.id)
          .map((p) => (
            <UICheckbox appearance="card"
              key={p.id}
              checked={taskIds.includes(p.id)}
              data-testid={"topic-task-" + p.id}
              onChange={(e) =>
                setTasks(
                  e.target.checked
                    ? [...taskIds, p.id]
                    : taskIds.filter((id) => id !== p.id),
                )
              }
            >
              {text(p.title)}
            </UICheckbox>
          ))}
      </div>
      <div className="judex-modal-actions">
        <Btn secondary onClick={onClose}>
          {t("cancel")}
        </Btn>
        <Btn
          testId="create-discussion"
          onClick={async () => {
            const r = await act("createDiscussion", {
              projectId: project.id,
              title,
              planIds,
              taskIds,
            });
            if (r.ok) {
              onClose();
              go({
                view: "topic",
                id: r.id,
              });
            }
          }}
        >
          {t("workNewDiscussion")}
        </Btn>
      </div>
    </Dialog>
  );
}
