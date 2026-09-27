import { UICard } from "../../components/ui/FormControls";
import { useWork } from "./store";
import { Heading, Btn, EmptyState, Person } from "./ui";
import { canAcceptTask, canOwnPlan, planTasks } from "./selectors";
import { PlanTile } from "./WorkPages";
import { TaskCard } from "./TaskCards";
import { planAction, decideDraft } from "./actions";
import { HandoffList } from "./HandoffPages";
export function DecisionsPage() {
  const { state, project, t, text, act, go } = useWork();
  const reviews = state.tasks.filter(
    (task) =>
      task.projectId === project.id &&
      task.status === "delivered" &&
      canAcceptTask(state, task),
  );
  const planReviews = state.plans.filter(
    (plan) =>
      plan.projectId === project.id &&
      plan.status === "active" &&
      canOwnPlan(state, plan) &&
      planTasks(state, plan.id).length > 0 &&
      planTasks(state, plan.id).every((task) => task.status === "accepted"),
  );
  const plans = state.plans.filter(
    (p) =>
      p.projectId === project.id &&
      p.status === "draft" &&
      canOwnPlan(state, p),
  );
  const tasks = state.tasks.filter(
    (task) =>
      task.projectId === project.id &&
      task.status === "draft" &&
      (task.planId
        ? state.plans.some((p) => p.id === task.planId && canOwnPlan(state, p))
        : project.members.some(
            (m) => m.name === state.currentUser && m.role === "owner",
          )),
  );
  return (
    <>
      <Heading
        eyebrow="A HUMAN DECISION MATTERS"
        title={t("cardsAllDecisions")}
        description={t("workDraftNote")}
      />
      <section className="judex-work-section">
        <h3>{t("workHandoffs")}</h3>
        <HandoffList compact />
      </section>
      {(reviews.length > 0 || planReviews.length > 0) && (
        <section className="judex-work-section">
          <h3>{t("cardsReviewSection")}</h3>
          <div className="judex-task-card-grid">
            {reviews.map((task) => (
              <TaskCard key={task.id} task={task} />
            ))}
            {planReviews.map((plan) => (
              <PlanTile key={plan.id} plan={plan} />
            ))}
          </div>
        </section>
      )}
      <div className="judex-decision-grid">
        {plans.map((p) => (
          <UICard className="judex-work-panel" key={p.id}>
            <span className="judex-eyebrow">{t("workPlans")}</span>
            <h2>{text(p.title)}</h2>
            <p>{text(p.goal)}</p>
            <div className="judex-task-actions">
              <Btn
                secondary
                onClick={() =>
                  go({
                    view: "plan",
                    id: p.id,
                  })
                }
              >
                {t("details")}
              </Btn>
              <Btn onClick={() => act((s) => planAction(s, p.id, "activate"))}>
                {t("workActivate")}
              </Btn>
            </div>
          </UICard>
        ))}
        {tasks.map((task) => (
          <UICard className="judex-work-panel" key={task.id}>
            <h2>{text(task.title)}</h2>
            <Person seatId={task.seatIds[0]} />
            <p>{text(task.expected)}</p>
            <div className="judex-task-actions">
              <Btn
                secondary
                onClick={() =>
                  go({
                    view: "task",
                    id: task.id,
                  })
                }
              >
                {t("details")}
              </Btn>
              <Btn onClick={() => act((s) => decideDraft(s, task.id))}>
                {t("workTaskDraftApprove")}
              </Btn>
            </div>
          </UICard>
        ))}
      </div>
      {!plans.length && !tasks.length && (
        <EmptyState text={t("workNoActions")} />
      )}
    </>
  );
}
