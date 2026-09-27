import {
  UIDisclosure,
  UIWarning,
  UIInput,
  UIOption,
  UISelect,
} from "../../components/ui/FormControls";
import { Button } from "../../components/ui/Button";
import { useState } from "react";
import { HandoffDialog } from "./CompositionDialogs";
import {
  ArrowLeft,
  ArrowRight,
  Plus,
  GitBranch,
  ClipboardList,
  Check,
  LockKeyhole,
  Sparkles,
  RotateCcw,
} from "lucide-react";
import { useWork } from "./store";
import {
  Btn,
  Dialog,
  EmptyState,
  EvidenceList,
  Heading,
  Person,
  Pill,
  TaskRow,
} from "./ui";
import {
  canAcceptTask,
  canOwnPlan,
  canWork,
  blockers,
  needsReview,
  planTasks,
  requirementMet,
  seatPerson,
} from "./selectors";
import { decideDraft, planAction, taskAction } from "./actions";
import {
  AcceptanceDialog,
  BriefDialog,
  CreateWorkDialog,
  ReasonDialog,
  ReportDialog,
} from "./Dialogs";
import { ExecutionMap } from "../chat/ExecutionMap";
import { TaskCard } from "./TaskCards";
import type { Plan, Task } from "./types";
export function PlanTile({
  plan,
  featured = false,
}: {
  plan: Plan;
  featured?: boolean;
}) {
  const { state, text, t, go } = useWork(),
    tasks = planTasks(state, plan.id),
    done = tasks.filter((t) => t.status === "accepted").length;
  return (
    <Button
      className={
        "judex-plan-tile" + (featured ? " judex-plan-tile-featured" : "")
      }
      onClick={() =>
        go({
          view: "plan",
          id: plan.id,
        })
      }
      data-testid={"plan-tile-" + plan.id}
    >
      <div className="judex-plan-tile-top">
        <span className="judex-eyebrow">{t("workDueLabel")}</span>
        <Pill status={plan.status} kind="plan" />
      </div>
      <h3>{text(plan.title)}</h3>
      <p>{text(plan.goal)}</p>
      <div className="judex-work-progress">
        <span
          style={{
            width: (tasks.length ? (done / tasks.length) * 100 : 0) + "%",
          }}
        />
      </div>
      <div className="judex-plan-tile-bottom">
        <Person seatId={plan.ownerSeatId} small />
        <small>
          {t("workPlanCount", {
            done,
            total: tasks.length,
          })}
        </small>
        <ArrowRight />
      </div>
    </Button>
  );
}
export function PlansPage() {
  const { state, project, t } = useWork(),
    [creating, setCreating] = useState(false);
  return (
    <>
      <Heading
        eyebrow="CHAPTERS OF A PROJECT"
        title={t("workPlans")}
        description={t("workNoAutoPlan")}
      >
        <Btn onClick={() => setCreating(true)}>
          <Plus />
          {t("workNewPlan")}
        </Btn>
      </Heading>
      <div className="judex-plan-grid">
        {state.plans
          .filter((p) => p.projectId === project.id)
          .map((p) => (
            <PlanTile key={p.id} plan={p} />
          ))}
      </div>
      {creating && (
        <CreateWorkDialog kind="plan" onClose={() => setCreating(false)} />
      )}
    </>
  );
}
export function PlanPage({ plan }: { plan: Plan }) {
  const { state, text, t, go, act } = useWork();
  const [creating, setCreating] = useState(false),
    [review, setReview] = useState(false),
    [selection, setSelection] = useState<string | null>(null),
    [parent, setParent] = useState<Task | undefined>();
  const tasks = planTasks(state, plan.id),
    done = tasks.filter((v) => v.status === "accepted").length,
    selected = tasks.find((v) => v.id === selection);
  return (
    <div className="judex-plan-page">
      <Button
        className="judex-work-back"
        onClick={() =>
          go({
            view: "plans",
          })
        }
      >
        <ArrowLeft />
        {t("workPlans")}
      </Button>
      <Heading
        eyebrow={t("workGoal")}
        title={text(plan.title)}
        description={text(plan.goal)}
      >
        <Pill status={plan.status} kind="plan" />
      </Heading>
      <div className="judex-plan-summary">
        <Person seatId={plan.ownerSeatId} />
        <div>
          <strong>
            {done} / {tasks.length}
          </strong>
          <small>{t("workTaskSummary")}</small>
        </div>
        <UIDisclosure title={<>{t("workCriteria")}</>}>
          {plan.criteria.map((c, i) => (
            <p key={i}>
              <Check />
              {text(c)}
            </p>
          ))}
        </UIDisclosure>
      </div>
      {needsReview(state, plan) && (
        <UIWarning
          className="judex-work-warning"
          data-testid="plan-reopened-warning"
        >
          <RotateCcw />
          {t("workReopenedPlan")}
          {canOwnPlan(state, plan) && (
            <Btn
              secondary
              onClick={() => act((s) => planAction(s, plan.id, "resume"))}
            >
              {t("workPlanResume")}
            </Btn>
          )}
        </UIWarning>
      )}
      <div className="judex-plan-next-step">
        <div>
          <span>{t("cardsNext")}</span>
          <p>
            {plan.status === "draft"
              ? t("cardsConfirm")
              : plan.status === "accepted"
                ? t("workPlanCompleted")
                : done === tasks.length && tasks.length > 0
                  ? t("cardsPlanReady")
                  : t("cardsPlanWait", {
                      count: tasks.length - done,
                    })}
          </p>
          <small>
            {t("cardsOwnerReview", {
              person: seatPerson(state, plan.ownerSeatId),
            })}
          </small>
        </div>
        <div>
          {canOwnPlan(state, plan) &&
            (plan.status === "draft" ? (
              <Btn
                testId="activate-plan"
                onClick={() => act((s) => planAction(s, plan.id, "activate"))}
              >
                {t("workActivate")}
              </Btn>
            ) : plan.status === "active" ? (
              <Btn
                testId="accept-plan"
                disabled={!tasks.length || done !== tasks.length}
                onClick={() => setReview(true)}
              >
                {t("workPlanAccept")}
              </Btn>
            ) : null)}
          <Btn
            secondary
            testId="new-plan-task"
            onClick={() => {
              setParent(undefined);
              setCreating(true);
            }}
          >
            <Plus />
            {t("workNewTask")}
          </Btn>
        </div>
      </div>
      <ExecutionMap tasks={tasks} onOpen={(task) => setSelection(task.id)} />
      {!!plan.referenceTaskIds.length && (
        <section className="judex-work-section">
          <h3>{t("workReferences")}</h3>
          <p className="judex-muted">{t("workReferenced")}</p>
          <div className="judex-task-card-grid">
            {plan.referenceTaskIds.map((id) => {
              const task = state.tasks.find((t) => t.id === id);
              return task && <TaskCard key={id} task={task} />;
            })}
          </div>
        </section>
      )}
      <p className="judex-work-small-note">{t("workNoAutoPlan")}</p>
      <RelatedTopics planId={plan.id} />
      <Activity targetId={plan.id} />
      {creating && (
        <CreateWorkDialog
          kind="task"
          parentTask={parent}
          onClose={() => setCreating(false)}
        />
      )}
      {review && (
        <AcceptanceDialog plan={plan} onClose={() => setReview(false)} />
      )}
      {selected && (
        <Dialog title={t("workTasks")} onClose={() => setSelection(null)} wide>
          <TaskPage task={selected} compact embedded />
          <Btn
            secondary
            onClick={() =>
              go({
                view: "task",
                id: selected.id,
              })
            }
          >
            {t("cardsOpenTask")}
            <ArrowRight />
          </Btn>
        </Dialog>
      )}
    </div>
  );
}
export function TasksPage() {
  const { t, state, project, text } = useWork();
  const [filter, setFilter] = useState("all"),
    [status, setStatus] = useState("all"),
    [query, setQuery] = useState(""),
    [creating, setCreating] = useState(false);
  const tasks = state.tasks.filter(
    (v) =>
      v.projectId === project.id &&
      (filter === "all" ||
        (filter === "direct" && !v.planId) ||
        (filter === "mine" && canWork(state, v))) &&
      (status === "all" || v.status === status) &&
      (!query.trim() ||
        [
          text(v.title),
          text(v.expected),
          ...v.seatIds.map((id) => seatPerson(state, id)),
        ]
          .join(" ")
          .toLocaleLowerCase()
          .includes(query.trim().toLocaleLowerCase())),
  );
  return (
    <>
      <Heading
        eyebrow="ONE OUTCOME AT A TIME"
        title={t("workTasks")}
        description={t("workNoReceiptIsAcceptance")}
      >
        <Btn onClick={() => setCreating(true)}>
          <Plus />
          {t("workNewTask")}
        </Btn>
      </Heading>
      <div className="judex-task-card-filters">
        <div className="judex-work-segments">
          {[
            ["all", "workAllTasks"],
            ["mine", "workHome"],
            ["direct", "workDirect"],
          ].map(([id, key]) => (
            <Button
              key={id}
              aria-pressed={filter === id}
              onClick={() => setFilter(id)}
            >
              {t(key as Parameters<typeof t>[0])}
            </Button>
          ))}
        </div>
        <UIInput
          className="judex-input"
          aria-label={t("cardsSearch")}
          placeholder={t("cardsSearch")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <UISelect
          className="judex-input"
          aria-label={t("cardsStatus")}
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        >
          <UIOption value="all">{t("cardsAllStatus")}</UIOption>
          {["draft", "ready", "working", "delivered", "rework", "accepted"].map(
            (v) => (
              <UIOption key={v} value={v}>
                {t(
                  v === "accepted"
                    ? "workTaskAccepted"
                    : (("workStatus" +
                        v[0].toUpperCase() +
                        v.slice(1)) as Parameters<typeof t>[0]),
                )}
              </UIOption>
            ),
          )}
        </UISelect>
      </div>
      <div className="judex-task-card-grid">
        {tasks.map((task) => (
          <TaskCard key={task.id} task={task} />
        ))}
      </div>
      {!tasks.length && <EmptyState text={t("workNoItems")} />}
      {creating && (
        <CreateWorkDialog kind="task" onClose={() => setCreating(false)} />
      )}
    </>
  );
}
export function TaskPage({
  task,
  compact = false,
  embedded = false,
}: {
  task: Task;
  compact?: boolean;
  embedded?: boolean;
}) {
  const { state, t, text, go, act } = useWork(),
    [tab, setTab] = useState("work"),
    [modal, setModal] = useState<
      "report" | "accept" | "reopen" | "brief" | "handoff" | null
    >(null);
  const plan = state.plans.find((p) => p.id === task.planId),
    blocked = blockers(state, task);
  const handoffs = state.handoffs.filter(
    (h) =>
      h.taskId === task.id || h.sources.some((src) => src.taskId === task.id),
  );
  return (
    <div
      className={
        "judex-task-page" + (compact ? " judex-task-page-compact" : "")
      }
      data-testid="task-detail"
    >
      {!embedded && (
        <Button
          className="judex-work-back"
          onClick={() =>
            go({
              view: compact ? "home" : plan ? "plan" : "tasks",
              id: compact ? undefined : plan?.id,
            })
          }
        >
          <ArrowLeft />
          {plan ? text(plan.title) : t("workTasks")}
        </Button>
      )}
      <Heading
        eyebrow={task.planId ? t("workTasks") : t("workDirect")}
        title={text(task.title)}
      >
        <Pill status={task.status} />
      </Heading>
      <div className="judex-task-people">
        {task.seatIds.map((id) => (
          <Person key={id} seatId={id} />
        ))}
        <span>
          {t("workReviewer")} · {seatPerson(state, task.reviewerSeatId)}
        </span>
      </div>
      <div className="judex-work-segments judex-work-detail-tabs">
        {[
          ["work", "workExpected"],
          ["evidence", "workEvidence"],
          ["discussion", "workRelatedTopics"],
          ["history", "workHistory"],
        ].map(([id, key]) => (
          <Button key={id} onClick={() => setTab(id)} aria-pressed={tab === id}>
            {t(key as Parameters<typeof t>[0])}
          </Button>
        ))}
      </div>
      {tab === "work" && (
        <>
          <section className="judex-task-agreement">
            <h2>{text(task.expected)}</h2>
            {task.criteria.map((c, i) => (
              <p key={i}>
                <span>{String(i + 1).padStart(2, "0")}</span>
                {text(c)}
              </p>
            ))}
          </section>
          {!!task.requirements.length && (
            <section className="judex-work-conditions">
              <h3>
                <LockKeyhole />
                {t("workHard")}
              </h3>
              {task.requirements.map((r) => (
                <div
                  key={r.id}
                  data-testid={"requirement-" + r.id}
                  className={
                    requirementMet(state, task, r)
                      ? "judex-check-met"
                      : "judex-check-missing"
                  }
                >
                  {requirementMet(state, task, r) ? <Check /> : <LockKeyhole />}
                  <span>{text(r.label)}</span>
                  <small>
                    {t(
                      requirementMet(state, task, r) ? "workMet" : "workUnmet",
                    )}
                  </small>
                </div>
              ))}
              <p>{t("workConditionHint")}</p>
            </section>
          )}
          {task.branch && (
            <p className="judex-work-branch">
              <GitBranch />
              {task.branch}
            </p>
          )}
          <div className="judex-task-actions">
            <Btn secondary onClick={() => setModal("brief")}>
              <ClipboardList />
              {t("workBrief")}
            </Btn>
            {canWork(state, task) &&
              ["delivered", "accepted"].includes(task.status) &&
              task.files.length > 0 && (
                <Btn
                  secondary
                  testId="propose-handoff"
                  onClick={() => setModal("handoff")}
                >
                  {t("workProposeHandoff")}
                </Btn>
              )}
            {task.status === "draft" &&
              (plan
                ? canOwnPlan(state, plan)
                : state.projects
                    .find((p) => p.id === task.projectId)
                    ?.members.some(
                      (m) => m.name === state.currentUser && m.role === "owner",
                    )) && (
                <Btn
                  testId="approve-task-draft"
                  onClick={() => act((s) => decideDraft(s, task.id))}
                >
                  {t("workTaskDraftApprove")}
                </Btn>
              )}
            {canWork(state, task) &&
              ["ready", "rework"].includes(task.status) && (
                <Btn
                  testId="start-task"
                  disabled={!!blockers(state, task, "start").length}
                  onClick={() => act((s) => taskAction(s, task.id, "start"))}
                >
                  {t("workTaskStart")}
                </Btn>
              )}
            {canWork(state, task) &&
              !["draft", "accepted"].includes(task.status) && (
                <Btn testId="report-task" onClick={() => setModal("report")}>
                  {t("workTaskReport")}
                </Btn>
              )}
            {canAcceptTask(state, task) && task.status === "delivered" && (
              <Btn
                testId="accept-task"
                disabled={!!blocked.length}
                onClick={() => setModal("accept")}
              >
                {t("workTaskAccept")}
              </Btn>
            )}
            {canAcceptTask(state, task) && task.status === "accepted" && (
              <Btn
                secondary
                testId="reopen-task"
                onClick={() => setModal("reopen")}
              >
                {t("workTaskReopen")}
              </Btn>
            )}
          </div>
          <p className="judex-work-small-note">{t("workLocalHint")}</p>
          {!!handoffs.length && (
            <section className="judex-work-section">
              <h3>{t("workHandoffs")}</h3>
              {handoffs.map((h) => (
                <Button
                  className="judex-linked-handoff"
                  key={h.id}
                  onClick={() =>
                    go({
                      view: "handoff",
                      id: h.id,
                    })
                  }
                >
                  <span>
                    <strong>{text(h.title)}</strong>
                    <small>
                      {t(
                        h.kind === "stage"
                          ? "workHandoffKindStage"
                          : "workHandoffKindDependency",
                      )}
                    </small>
                  </span>
                  <ArrowRight />
                </Button>
              ))}
            </section>
          )}
        </>
      )}
      {tab === "evidence" && <EvidenceList files={task.files} />}
      {tab === "discussion" && <RelatedTopics taskId={task.id} />}
      {tab === "history" && <Activity targetId={task.id} />}
      {modal === "handoff" && (
        <HandoffDialog task={task} onClose={() => setModal(null)} />
      )}
      {modal === "report" && (
        <ReportDialog task={task} onClose={() => setModal(null)} />
      )}
      {modal === "brief" && (
        <BriefDialog task={task} onClose={() => setModal(null)} />
      )}
      {modal === "accept" && (
        <AcceptanceDialog task={task} onClose={() => setModal(null)} />
      )}
      {modal === "reopen" && (
        <ReasonDialog task={task} onClose={() => setModal(null)} />
      )}
    </div>
  );
}
export function RelatedTopics({
  taskId,
  planId,
}: {
  taskId?: string;
  planId?: string;
}) {
  const { state, project, t, text, go } = useWork(),
    topics = state.topics.filter(
      (v) =>
        v.projectId === project.id &&
        (taskId
          ? v.taskIds.includes(taskId)
          : planId
            ? v.planIds.includes(planId)
            : true),
    );
  return (
    <section className="judex-work-section">
      <h3>{t("workRelatedTopics")}</h3>
      {topics.map((topic) => (
        <Button
          className="judex-linked-handoff"
          key={topic.id}
          onClick={() =>
            go({
              view: "topic",
              id: topic.id,
            })
          }
        >
          <span>
            <strong>{text(topic.title)}</strong>
            <small>{t("workPureTopic")}</small>
          </span>
          <ArrowRight />
        </Button>
      ))}
      {!topics.length && <p className="judex-muted">{t("workNoItems")}</p>}
    </section>
  );
}
export function Activity({ targetId }: { targetId: string }) {
  const { state, t, text, locale } = useWork(),
    events = state.events
      .filter((e) => e.targetId === targetId)
      .slice()
      .reverse();
  return (
    <section className="judex-work-section judex-work-activity">
      <h3>{t("workHistory")}</h3>
      {events.map((e) => (
        <article key={e.id}>
          <span className="judex-activity-point" />
          <div>
            <strong>{e.actor}</strong>
            <small>{new Date(e.at).toLocaleString(locale)}</small>
            <p>{text(e.text)}</p>
          </div>
        </article>
      ))}
      {!events.length && <p className="judex-muted">{t("workNoHistory")}</p>}
    </section>
  );
}
