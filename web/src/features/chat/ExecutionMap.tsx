import { UIWarning } from "../../components/ui/FormControls";
import { useId, useState, type CSSProperties } from "react";
import { ArrowRight, Maximize2, Minus, Plus, GitBranch } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { useWork } from "../work/store";
import { Dialog, Person, Pill } from "../work/ui";
import type { Task } from "../work/types";
import { executionLayout, GRAPH } from "./executionLayout";
export function ExecutionMap({
  tasks,
  onOpen,
}: {
  tasks: Task[];
  onOpen: (task: Task) => void;
}) {
  const { state, t, text } = useWork();
  const [zoom, setZoom] = useState(0.8),
    [wide, setWide] = useState(false);
  const arrow = "dep" + useId().replaceAll(":", "");
  const graph = executionLayout(tasks, state.handoffs);
  const content = (
    <>
      <div className="judex-execution-toolbar">
        <span>
          <GitBranch size={16} />
          {t("chatOrder")}
        </span>
        <div>
          <Button
            aria-label={t("chatZoomOut")}
            onClick={() => setZoom((v) => Math.max(0.4, v - 0.1))}
          >
            <Minus size={15} />
          </Button>
          <small>{Math.round(zoom * 100)}%</small>
          <Button
            aria-label={t("chatZoomIn")}
            onClick={() => setZoom((v) => Math.min(1.2, v + 0.1))}
          >
            <Plus size={15} />
          </Button>
          {!wide && (
            <Button
              aria-label={t("chatFullGraph")}
              onClick={() => setWide(true)}
            >
              <Maximize2 size={15} />
            </Button>
          )}
        </div>
      </div>
      <p className="judex-chat-footnote">
        {t("chatOrderHint")} {t("chatGraphConditions")}
      </p>
      {graph.invalid.length > 0 && (
        <UIWarning className="judex-work-warning">
          {t("chatCycle")} {graph.invalid.join(", ")}
        </UIWarning>
      )}
      {!tasks.length ? (
        <div className="judex-chat-empty">
          <GitBranch />
          <p>{t("chatEmptyGraph")}</p>
        </div>
      ) : (
        <div className="judex-execution-viewport" data-testid="execution-map">
          <div
            style={{
              width: graph.width * zoom,
              height: (graph.height + 40) * zoom,
            }}
          >
            <div
              className="judex-execution-canvas"
              style={
                {
                  width: graph.width,
                  height: graph.height + 40,
                  transform: `scale(${zoom})`,
                  "--graph-card-width": GRAPH.width + "px",
                  "--graph-card-height": GRAPH.height + "px",
                } as CSSProperties
              }
            >
              {[
                ...new Set([...graph.positions.values()].map((p) => p.rank)),
              ].map((rank) => (
                <div
                  className="judex-execution-stage"
                  style={{
                    left: GRAPH.pad + rank * GRAPH.column,
                  }}
                  key={rank}
                >
                  {t("chatStage", {
                    n: rank + 1,
                  })}
                  <ArrowRight size={14} />
                </div>
              ))}
              <svg
                width={graph.width}
                height={graph.height + 40}
                aria-hidden="true"
              >
                <defs>
                  <marker
                    id={arrow}
                    viewBox="0 0 10 10"
                    refX="8"
                    refY="5"
                    markerWidth="6"
                    markerHeight="6"
                    orient="auto"
                  >
                    <path d="M 0 0 L 10 5 L 0 10 z" />
                  </marker>
                </defs>
                {graph.edges.map((e) => {
                  const a = graph.positions.get(e.from)!,
                    b = graph.positions.get(e.to)!;
                  const x = a.x + GRAPH.width,
                    y = a.y + 40 + GRAPH.height / 2,
                    end = b.y + 40 + GRAPH.height / 2;
                  return (
                    <path
                      className={
                        e.hard
                          ? "judex-execution-edge"
                          : "judex-execution-edge judex-execution-edge-soft"
                      }
                      key={e.from + e.to}
                      data-from={e.from}
                      data-to={e.to}
                      markerEnd={`url(#${arrow})`}
                      d={`M ${x} ${y} C ${x + 36} ${y},${b.x - 36} ${end},${b.x} ${end}`}
                    />
                  );
                })}
              </svg>
              {tasks.map((task) => {
                const p = graph.positions.get(task.id)!;
                return (
                  <Button
                    key={task.id}
                    data-testid={"execution-task-" + task.id}
                    data-rank={p.rank}
                    className="judex-execution-card"
                    style={{
                      left: p.x,
                      top: p.y + 40,
                    }}
                    onClick={() => onOpen(task)}
                  >
                    <Pill status={task.status} />
                    <strong>{text(task.title)}</strong>
                    {task.requirements.some((r) => r.kind !== "evidence") && (
                      <small>
                        {t(
                          task.requirements.some(
                            (r) => r.hard && r.at !== "accept",
                          )
                            ? "chatBeforeStart"
                            : "chatBeforeAccept",
                        )}
                      </small>
                    )}
                    <Person seatId={task.seatIds[0]} small />
                    {task.parentId && (
                      <small>
                        {t("chatParent")} ·{" "}
                        {text(
                          state.tasks.find((t) => t.id === task.parentId)
                            ?.title ?? task.parentId,
                        )}
                      </small>
                    )}
                    {graph.external.has(task.id) && (
                      <small>{t("chatExternal")}</small>
                    )}
                  </Button>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </>
  );
  return (
    <section className="judex-execution-map">
      {wide ? (
        <Dialog title={t("chatRoadmap")} wide onClose={() => setWide(false)}>
          {content}
        </Dialog>
      ) : (
        content
      )}
    </section>
  );
}
