import { UIOption, UISelect } from "../../components/ui/FormControls";
import { useState } from "react";
import {
  ArrowUpRight,
  Check,
  GitBranch,
  Users,
  SlidersHorizontal,
  Terminal,
  Plus,
} from "lucide-react";
import { useWork } from "../work/store";
import { Button } from "../../components/ui/Button";
import { Btn, Person, Pill } from "../work/ui";
import { PlanPage, TaskPage } from "../work/WorkPages";
import { HandoffList } from "../work/HandoffPages";
import { ResourcesPage } from "../work/ProjectPages";
import { CreateWorkDialog } from "../work/Dialogs";
import { DecisionsPage } from "../work/DecisionsPage";
import {
  canOwnPlan,
  planTasks,
  sourceActions,
  canAcceptTask,
} from "../work/selectors";
import { ExecutionMap } from "./ExecutionMap";
import { ProposalCard } from "./ProposalCard";
import { useWorkspaceDraft } from "./useWorkspaceDraft";
export function WorkMap() {
  const { state, project, t, text, go } = useWork();
  const [planId, setPlan] = useWorkspaceDraft("work-map-plan", ""),
    [creating, setCreating] = useState<"plan" | "task" | null>(null);
  const plans = state.plans.filter((p) => p.projectId === project.id),
    tasks = state.tasks.filter(
      (v) => v.projectId === project.id && (!planId || v.planId === planId),
    );
  return (
    <>
      <div className="judex-chat-panel-title">
        <div>
          <small>{t("chatRoadmap")}</small>
          <h2>{t("chatOrder")}</h2>
        </div>
        <Button
          aria-label={t("workNewPlan")}
          onClick={() => setCreating("plan")}
        >
          <Plus />
        </Button>
      </div>
      <div className="judex-chat-map-filter">
        <UISelect
          data-testid="work-map-plan-filter"
          className="judex-input"
          aria-label={t("workPlans")}
          value={planId}
          onChange={(e) => setPlan(e.target.value)}
        >
          <UIOption value="">{t("chatAllPlans")}</UIOption>
          {plans.map((p) => (
            <UIOption key={p.id} value={p.id}>
              {text(p.title)}
            </UIOption>
          ))}
        </UISelect>
        <Btn secondary onClick={() => setCreating("task")}>
          {t("workNewTask")}
        </Btn>
      </div>
      {plans
        .filter((p) => !planId || p.id === planId)
        .map((p) => (
          <Button
            className="judex-chat-plan-strip"
            key={p.id}
            onClick={() =>
              go({
                view: "plan",
                id: p.id,
              })
            }
          >
            <span>
              <strong>{text(p.title)}</strong>
              <small>
                {state.seats.find((s) => s.id === p.ownerSeatId)?.person} ·{" "}
                {
                  planTasks(state, p.id).filter((t) => t.status === "accepted")
                    .length
                }
                /{planTasks(state, p.id).length}
              </small>
            </span>
            <Pill status={p.status} kind="plan" />
            <ArrowUpRight size={15} />
          </Button>
        ))}
      <ExecutionMap
        tasks={tasks}
        onOpen={(task) =>
          go({
            view: "task",
            id: task.id,
          })
        }
      />
      {creating && (
        <CreateWorkDialog kind={creating} onClose={() => setCreating(null)} />
      )}
    </>
  );
}
export function NowPanel() {
  const { state, project, t, text, go } = useWork(),
    flows = state.flows.filter((f) => f.projectId === project.id),
    seats = state.seats.filter((s) =>
      state.positions.some(
        (p) => p.projectId === project.id && p.id === s.positionId,
      ),
    );
  const items = sourceActions(state, project.id),
    reviews = state.tasks.filter(
      (v) =>
        v.projectId === project.id &&
        v.status === "delivered" &&
        canAcceptTask(state, v),
    );
  const plans = state.plans.filter(
    (p) =>
      p.projectId === project.id &&
      p.status === "active" &&
      canOwnPlan(state, p) &&
      planTasks(state, p.id).length > 0 &&
      planTasks(state, p.id).every((t) => t.status === "accepted"),
  );
  const proposals =
    state.proposals?.filter(
      (p) =>
        p.projectId === project.id &&
        p.status === "pending" &&
        p.approvers.includes(state.currentUser) &&
        !p.votes[state.currentUser],
    ) ?? [];
  return (
    <>
      <div className="judex-chat-panel-title">
        <div>
          <small>{t("chatWorkspace")}</small>
          <h2>{t("chatNow")}</h2>
        </div>
        <span className="judex-chat-little-star">✳</span>
      </div>
      {(!flows.length || !seats.length) && (
        <section className="judex-chat-setup">
          <h3>{t("chatSetup")}</h3>
          <p>{t("chatSetupHint")}</p>
          {[
            ["flows", "chatSetupFlow", GitBranch],
            ["team", "chatSetupTeam", Users],
            ["settings", "chatSetupPrompt", SlidersHorizontal],
          ].map(([view, label, Icon]: any) => (
            <Button
              key={view}
              onClick={() =>
                go({
                  view,
                })
              }
            >
              <Icon size={17} />
              {t(label)}
              <ArrowUpRight size={15} />
            </Button>
          ))}
        </section>
      )}
      {items.map((item) => (
        <Button
          className="judex-chat-action-card"
          key={item.handoff.id + item.source.id}
          onClick={() =>
            go({
              view: "handoff",
              id: item.handoff.id,
            })
          }
        >
          <Person seatId={item.source.senderSeatId} />
          <strong>{text(item.handoff.title)}</strong>
          <small>
            {t("workHandoffs")}
            <ArrowUpRight size={14} />
          </small>
        </Button>
      ))}
      {reviews.map((task) => (
        <Button
          key={task.id}
          className="judex-chat-action-card"
          onClick={() =>
            go({
              view: "task",
              id: task.id,
            })
          }
        >
          <small>{t("workTaskAccepted")}</small>
          <strong>{text(task.title)}</strong>
          <ArrowUpRight size={15} />
        </Button>
      ))}
      {plans.map((plan) => (
        <Button
          className="judex-chat-action-card"
          key={plan.id}
          data-testid={"today-plan-review-" + plan.id}
          onClick={() =>
            go({
              view: "plan",
              id: plan.id,
            })
          }
        >
          <Check />
          <strong>{text(plan.title)}</strong>
          <small>{t("workNoAutoPlan")}</small>
        </Button>
      ))}
      {proposals.map((p) => (
        <Button
          className="judex-chat-action-card"
          key={p.id}
          onClick={() =>
            go({
              view: "topic",
              id: p.topicId,
            })
          }
        >
          <span>
            {t("chatProposal")} · v{p.revision}
          </span>
          <strong>{p.title}</strong>
          <small>{t("chatWaiting")}</small>
        </Button>
      ))}
      {!items.length &&
        !reviews.length &&
        !plans.length &&
        !proposals.length && (
          <p className="judex-chat-empty">{t("chatNoActions")}</p>
        )}
      <div className="judex-chat-context-title">{t("chatPeople")}</div>
      <div className="judex-chat-partners">
        {seats.slice(0, 5).map((s) => (
          <Person key={s.id} seatId={s.id} />
        ))}
      </div>
      <Button
        className="judex-chat-local-link"
        onClick={() =>
          go({
            view: "settings",
            id: "local",
          })
        }
      >
        <Terminal size={16} />
        {t("chatLocal")}
        <ArrowUpRight size={14} />
      </Button>
    </>
  );
}
export function WorkspacePanel() {
  const { state, route, project, t, go } = useWork();
  const task = state.tasks.find(
      (t) => t.id === route.id && t.projectId === project.id,
    ),
    plan = state.plans.find(
      (p) => p.id === route.id && p.projectId === project.id,
    );
  switch (route.view) {
    case "plan":
      return plan ? <PlanPage key={plan.id} plan={plan} /> : <WorkMap />;
    case "task":
      return task ? (
        <TaskPage key={task.id} task={task} compact />
      ) : (
        <WorkMap />
      );
    case "plans":
    case "tasks":
      return <WorkMap />;
    case "resources":
      return <ResourcesPage />;
    case "handoffs":
      return <HandoffList />;
    case "decisions":
      return (
        <>
          {state.proposals
            ?.filter(
              (p) =>
                p.projectId === project.id &&
                p.status === "pending" &&
                p.approvers.includes(state.currentUser),
            )
            .map((p) => (
              <ProposalCard key={p.id} proposal={p} />
            ))}
          <DecisionsPage />
        </>
      );
    default:
      return <NowPanel />;
  }
}
