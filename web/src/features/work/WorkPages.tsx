import {Button} from '../../components/ui/Button';
import {ArrowRight} from 'lucide-react';
import {useWork} from './store';
import {Person,Pill} from './ui';
import {planTasks} from './selectors';
import type {Plan} from './types';
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
