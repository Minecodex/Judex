import {
  UIOption,
  UISelect,
  UICheckbox,
} from "../../components/ui/FormControls";
import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Input, TextArea } from "@heroui/react";
import { useWork } from "../work/store";
import { Btn, Dialog, Field } from "../work/ui";
import { Button } from "../../components/ui/Button";
import { type ProposalInput, type WorkProposal } from "./proposalModel";
export function ProposalDialog({
  topicId,
  onClose,
  previous,
}: {
  topicId: string;
  onClose: () => void;
  previous?: WorkProposal;
}) {
  const { state, project, t, text, act, go } = useWork();
  const topic = state.topics.find((v) => v.id === topicId)!;
  const flows = state.flows.filter((f) => f.projectId === project.id),
    seats = state.seats.filter((s) =>
      state.positions.some(
        (p) => p.id === s.positionId && p.projectId === project.id,
      ),
    );
  const candidates = (flowId: string, nodeId: string) =>
    seats.filter((s) =>
      state.positions.some(
        (p) =>
          p.id === s.positionId &&
          p.bindings.some((b) => b.flowId === flowId && b.nodeId === nodeId),
      ),
    );
  const defaults = (flowId: string) => {
    const flow = flows.find((f) => f.id === flowId);
    return (
      flow?.nodes.map((node) => ({
        title: text(node.label),
        nodeId: node.id,
        seatId: candidates(flowId, node.id)[0]?.id ?? "",
        reviewerSeatId: seats[0]?.id ?? "",
        after: [] as number[],
      })) ?? []
    );
  };
  const [draft, setDraft] = useState<ProposalInput>(
    previous
      ? {
          title: previous.title,
          goal: previous.goal,
          criteria: previous.criteria,
          ownerSeatId: previous.ownerSeatId,
          flowId: previous.flowId,
          tasks: structuredClone(previous.tasks),
        }
      : {
          title: text(topic.title),
          goal: text(
            topic.messages.filter((m) => m.kind === "person").at(-1)?.text ??
              topic.title,
          ),
          criteria: "",
          ownerSeatId:
            seats.find((s) => s.person === state.currentUser)?.id ??
            seats[0]?.id ??
            "",
          flowId: flows[0]?.id ?? "",
          tasks: defaults(flows[0]?.id ?? ""),
        },
  );
  const set = (patch: Partial<ProposalInput>) =>
    setDraft({
      ...draft,
      ...patch,
    });
  const patchTask = (
    i: number,
    patch: Partial<ProposalInput["tasks"][number]>,
  ) =>
    set({
      tasks: draft.tasks.map((v, n) =>
        n === i
          ? {
              ...v,
              ...patch,
            }
          : v,
      ),
    });
  const options = (items = seats) =>
    items.map((s) => (
      <UIOption key={s.id} value={s.id}>
        {s.person} ·{" "}
        {text(state.positions.find((p) => p.id === s.positionId)!.name)}
      </UIOption>
    ));
  const flow = flows.find((f) => f.id === draft.flowId);
  const people = [
    ...new Set(
      [
        draft.ownerSeatId,
        ...draft.tasks.flatMap((v) => [v.seatId, v.reviewerSeatId]),
      ]
        .map((id) => seats.find((s) => s.id === id)?.person)
        .filter(Boolean),
    ),
  ];
  return (
    <Dialog title={t("chatProposal")} onClose={onClose} wide>
      <p className="judex-modal-description">{t("chatProposalHint")}</p>
      {!flows.length || !seats.length ? (
        <>
          <p>{t("chatNeedSetup")}</p>
          <Btn
            onClick={() => {
              onClose();
              go({
                view: "flows",
              });
            }}
          >
            {t("chatFlows")}
          </Btn>
        </>
      ) : (
        <>
          <Field label={t("chatTitle")}>
            <Input
              aria-label={t("chatTitle")}
              data-testid="proposal-title"
              value={draft.title}
              onChange={(e) =>
                set({
                  title: e.target.value,
                })
              }
            />
          </Field>
          <Field label={t("chatGoal")}>
            <TextArea
              aria-label={t("chatGoal")}
              value={draft.goal}
              onChange={(e) =>
                set({
                  goal: e.target.value,
                })
              }
            />
          </Field>
          <Field label={t("chatCriteria")}>
            <TextArea
              aria-label={t("chatCriteria")}
              data-testid="proposal-criteria"
              value={draft.criteria}
              onChange={(e) =>
                set({
                  criteria: e.target.value,
                })
              }
            />
          </Field>
          <div className="judex-work-form-grid">
            <Field label={t("chatFlow")}>
              <UISelect
                className="judex-input"
                value={draft.flowId}
                onChange={(e) =>
                  set({
                    flowId: e.target.value,
                    tasks: defaults(e.target.value),
                  })
                }
              >
                {flows.map((f) => (
                  <UIOption key={f.id} value={f.id}>
                    {text(f.name)} · v{f.version}
                  </UIOption>
                ))}
              </UISelect>
            </Field>
            <Field label={t("chatOwner")}>
              <UISelect
                className="judex-input"
                value={draft.ownerSeatId}
                onChange={(e) =>
                  set({
                    ownerSeatId: e.target.value,
                  })
                }
              >
                {options()}
              </UISelect>
            </Field>
          </div>
          <div className="judex-chat-proposed-tasks">
            {draft.tasks.map((task, i) => (
              <section key={i}>
                <header>
                  <b>{i + 1}</b>
                  <Input
                    aria-label={t("chatTaskTitle") + " " + (i + 1)}
                    value={task.title}
                    onChange={(e) =>
                      patchTask(i, {
                        title: e.target.value,
                      })
                    }
                  />
                  <Button
                    aria-label={t("chatRemoveTask")}
                    disabled={draft.tasks.length === 1}
                    onClick={() =>
                      set({
                        tasks: draft.tasks
                          .filter((_, n) => n !== i)
                          .map((v) => ({
                            ...v,
                            after: v.after
                              .filter((n) => n !== i)
                              .map((n) => (n > i ? n - 1 : n)),
                          })),
                      })
                    }
                  >
                    <Trash2 size={16} />
                  </Button>
                </header>
                <div className="judex-work-form-grid">
                  <Field label={t("chatNode")}>
                    <UISelect
                      className="judex-input"
                      value={task.nodeId}
                      onChange={(e) =>
                        patchTask(i, {
                          nodeId: e.target.value,
                          seatId:
                            candidates(draft.flowId, e.target.value)[0]?.id ??
                            "",
                        })
                      }
                    >
                      {flow?.nodes.map((n) => (
                        <UIOption key={n.id} value={n.id}>
                          {text(n.label)}
                        </UIOption>
                      ))}
                    </UISelect>
                  </Field>
                  <Field label={t("chatAssignee")}>
                    <UISelect
                      className="judex-input"
                      value={task.seatId}
                      onChange={(e) =>
                        patchTask(i, {
                          seatId: e.target.value,
                        })
                      }
                    >
                      <UIOption value="">—</UIOption>
                      {options(candidates(draft.flowId, task.nodeId))}
                    </UISelect>
                  </Field>
                  <Field label={t("chatReviewer")}>
                    <UISelect
                      className="judex-input"
                      value={task.reviewerSeatId}
                      onChange={(e) =>
                        patchTask(i, {
                          reviewerSeatId: e.target.value,
                        })
                      }
                    >
                      {options()}
                    </UISelect>
                  </Field>
                </div>
                <fieldset>
                  <legend>{t("chatAfter")}</legend>
                  {!i && <small>{t("chatParallel")}</small>}
                  {draft.tasks.slice(0, i).map((v, n) => (
                    <UICheckbox
                      key={n}
                      checked={task.after.includes(n)}
                      onChange={(e) =>
                        patchTask(i, {
                          after: e.target.checked
                            ? [...task.after, n]
                            : task.after.filter((x) => x !== n),
                        })
                      }
                    >
                      {v.title}
                    </UICheckbox>
                  ))}
                </fieldset>
              </section>
            ))}
          </div>
          <Btn
            secondary
            onClick={() =>
              set({
                tasks: [
                  ...draft.tasks,
                  {
                    ...defaults(draft.flowId)[0],
                    title: "",
                    after: [],
                  },
                ],
              })
            }
          >
            <Plus size={15} />
            {t("chatAddTask")}
          </Btn>
          <p>
            {t("chatApprovers")}：{people.join("、")}
          </p>
          <Btn
            testId="send-proposal"
            disabled={
              !draft.title.trim() ||
              !draft.criteria.trim() ||
              draft.tasks.some((v) => !v.seatId || !v.title.trim())
            }
            onClick={async () => {
              if (
                (
                  await act("submitProposal", {
                    topicId,
                    draft,
                    previousId: previous?.id,
                  })
                ).ok
              )
                onClose();
            }}
          >
            {t("chatSendProposal")}
          </Btn>
        </>
      )}
    </Dialog>
  );
}
