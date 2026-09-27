import { Button } from "../../components/ui/Button";
import { ArrowUpRight, FileText, GitBranch, LockKeyhole } from "lucide-react";
import { useWork } from "./store";
import { blockers } from "./selectors";
import { Person, Pill } from "./ui";
import type { Task } from "./types";
export function taskNext(task: Task) {
  return task.status === "draft"
    ? "cardsConfirm"
    : task.status === "ready"
      ? "cardsStart"
      : task.status === "working"
        ? "cardsReport"
        : task.status === "rework"
          ? "cardsRevise"
          : task.status === "delivered"
            ? "cardsReview"
            : "cardsDone";
}
export function TaskCard({
  task,
  onOpen,
  compact = false,
}: {
  task: Task;
  onOpen?: () => void;
  compact?: boolean;
}) {
  const { state, t, text, go } = useWork();
  const blocked = blockers(
    state,
    task,
    task.status === "ready" ? "start" : "accept",
  );
  const plan = state.plans.find((p) => p.id === task.planId);
  const children = state.tasks.filter((v) => v.parentId === task.id);
  return (
    <Button
      className={
        "judex-task-card" + (compact ? " judex-task-card-compact" : "")
      }
      data-testid={"work-task-" + task.id}
      onClick={onOpen ?? (() => go({ view: "task", id: task.id }))}
    >
      <div className="judex-task-card-top">
        <Pill status={task.status} />
        <ArrowUpRight />
      </div>
      <h3>{text(task.title)}</h3>
      {!compact && (
        <p className="judex-task-card-description">{text(task.expected)}</p>
      )}
      <div className="judex-task-card-owner">
        <Person seatId={task.seatIds[0]} small />
        <span>
          {blocked.length && task.status !== "accepted" ? (
            <>
              <LockKeyhole />
              {t("cardsBlocked", { count: blocked.length })}
            </>
          ) : (
            t(taskNext(task))
          )}
        </span>
      </div>
      {!compact && (
        <>
          <div className="judex-task-card-meta">
            <span>
              <FileText />
              {task.files.length} {t("workEvidence")}
            </span>
            {children.length > 0 && (
              <span>
                <GitBranch />
                {t("cardsChildren", { count: children.length })}
              </span>
            )}
          </div>
          <div className="judex-task-card-foot">
            <span>{plan ? text(plan.title) : t("workDirect")}</span>
            <small>
              {t("workReviewer")} ·{" "}
              {state.seats.find((s) => s.id === task.reviewerSeatId)?.person}
            </small>
          </div>
          {blocked.length > 0 && task.status !== "accepted" && (
            <p className="judex-task-card-blocker">{text(blocked[0].label)}</p>
          )}
        </>
      )}
    </Button>
  );
}
