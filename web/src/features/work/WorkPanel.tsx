import { UIWarning } from "../../components/ui/FormControls";
import { Button, Card, Input } from "@heroui/react";
import { Plus, X } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";
import { errorText } from "../projects/ProjectWorkspace";

type Plan = { id: string; title: string; status: string; version: number; taskStats: { total: number; accepted: number } };
type Task = { id: string; title: string; status: string; version: number; planId: string | null };
type Proposal = { id: string; kind: string; status: string; version: number };
type Review = {
  reviewId: string;
  reviewHash: string;
  status: string;
  slots: { id: string; displayName: string; state: string; positionName: string | null }[];
  deadlineAt: string | null;
};

// WorkPanel（P3-10）：计划/任务/提案/待办的真实 API 面板。所有命令携带
// expectedVersion 与幂等键；批准/验收不做乐观更新。
export function WorkPanel({ projectId }: { projectId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [creatingPlan, setCreatingPlan] = useState(false);
  const [planTitle, setPlanTitle] = useState("");
  const [selectedPlan, setSelectedPlan] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<string | null>(null);

  const plans = useQuery({
    queryKey: ["plans", projectId],
    queryFn: () => request<{ items: Plan[] }>(`/projects/${projectId}/plans`),
  });
  const actions = useQuery({
    queryKey: ["actions", projectId],
    queryFn: () => request<{ items: { kind: string; objectType: string; objectId: string; summary: string }[] }>(
      `/me/actions?projectId=${projectId}`,
    ),
    refetchInterval: 8000,
  });
  const tasks = useQuery({
    queryKey: ["tasks", projectId, selectedPlan ?? ""],
    queryFn: () =>
      request<{ items: Task[] }>(`/projects/${projectId}/tasks${selectedPlan ? `?planId=${selectedPlan}` : ""}`),
    enabled: !!selectedPlan || true,
  });

  const createPlan = useMutation({
    mutationFn: () =>
      request(`/projects/${projectId}/plans`, {
        method: "POST",
        body: JSON.stringify({ title: planTitle.trim() }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: () => {
      setPlanTitle("");
      setCreatingPlan(false);
      client.invalidateQueries({ queryKey: ["plans", projectId] });
    },
  });

  return (
    <div className="judex-panel-stack">
      <div className="judex-workspace-section">
        <h3>{t("wpPlans")}</h3>
        <Button onClick={() => setCreatingPlan((v) => !v)}>
          <Plus size={15} />
          {t("wpNewPlan")}
        </Button>
      </div>
      {creatingPlan && (
        <form
          className="judex-inline-form"
          onSubmit={(event) => {
            event.preventDefault();
            if (planTitle.trim()) createPlan.mutate();
          }}
        >
          <Input aria-label={t("wpNewPlan")} value={planTitle} onChange={(e) => setPlanTitle(e.target.value)} autoFocus />
          <Button type="submit" isPending={createPlan.isPending}>
            {createPlan.isPending ? t("submitting") : t("pwCreate")}
          </Button>
          <Button variant="ghost" onClick={() => setCreatingPlan(false)}>
            <X size={15} />
          </Button>
          {createPlan.isError && <span role="alert">{errorText(locale, createPlan.error)}</span>}
        </form>
      )}
      {plans.isPending ? (
        <p className="judex-workspace-status">{t("shellLoading")}</p>
      ) : plans.isError ? (
        <UIWarning role="alert">{errorText(locale, plans.error)}</UIWarning>
      ) : (
        <ul className="judex-workspace-list">
          {(plans.data?.items ?? []).map((plan) => (
            <li key={plan.id} className="judex-workspace-item">
              <Button variant={selectedPlan === plan.id ? "secondary" : "ghost"} onClick={() => setSelectedPlan(plan.id)}>
                {plan.title}
              </Button>
              <small>
                {plan.status} · {plan.taskStats?.accepted ?? 0}/{plan.taskStats?.total ?? 0}
              </small>
            </li>
          ))}
          {(plans.data?.items ?? []).length === 0 && (
            <li className="judex-workspace-status">{t("wpNoPlans")}</li>
          )}
        </ul>
      )}
      {selectedPlan && <TaskList projectId={projectId} planId={selectedPlan} onOpen={setSelectedTask} />}
      {selectedTask && <TaskDetail projectId={projectId} taskId={selectedTask} />}
      <div className="judex-workspace-section">
        <h3>{t("wpTodos")}</h3>
      </div>
      {actions.isPending ? (
        <p className="judex-workspace-status">{t("shellLoading")}</p>
      ) : (
        <ul className="judex-workspace-list">
          {(actions.data?.items ?? []).map((action, index) => (
            <li key={index} className="judex-workspace-item">
              <small>{action.summary}</small>
              <small>
                {action.objectType}/{action.kind}
              </small>
            </li>
          ))}
          {(actions.data?.items ?? []).length === 0 && (
            <li className="judex-workspace-status">{t("wpNoTodos")}</li>
          )}
        </ul>
      )}
      <ProposalList projectId={projectId} />
    </div>
  );
}

function TaskList({ projectId, planId, onOpen }: { projectId: string; planId: string; onOpen: (id: string) => void }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const tasks = useQuery({
    queryKey: ["tasks", projectId, planId],
    queryFn: () => request<{ items: Task[] }>(`/projects/${projectId}/tasks?planId=${planId}`),
  });
  if (tasks.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  return (
    <ul className="judex-workspace-list">
      {(tasks.data?.items ?? []).map((task) => (
        <li key={task.id} className="judex-workspace-item">
          <Button variant="ghost" onClick={() => onOpen(task.id)}>
            {task.title}
          </Button>
          <small>
            {task.status} · v{task.version}
          </small>
        </li>
      ))}
      {(tasks.data?.items ?? []).length === 0 && <li className="judex-workspace-status">{t("wpNoTasks")}</li>}
    </ul>
  );
}

function TaskDetail({ projectId, taskId }: { projectId: string; taskId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const tasks = useQuery({
    queryKey: ["tasks", projectId, ""],
    queryFn: () => request<{ items: Task[] }>(`/projects/${projectId}/tasks`),
  });
  const task = tasks.data?.items?.find((item) => item.id === taskId);
  const invalidate = () => {
    client.invalidateQueries({ queryKey: ["tasks", projectId] });
    client.invalidateQueries({ queryKey: ["actions", projectId] });
  };
  const start = useMutation({
    mutationFn: () =>
      request(`/projects/${projectId}/tasks/${taskId}/start`, {
        method: "POST",
        body: JSON.stringify({ expectedVersion: task?.version ?? 1 }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: invalidate,
  });
  const deliver = useMutation({
    mutationFn: () =>
      request(`/projects/${projectId}/tasks/${taskId}/reports`, {
        method: "POST",
        body: JSON.stringify({ kind: "delivery", text: "交付", expectedTaskVersion: task?.version ?? 1 }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: invalidate,
  });
  const accept = useMutation({
    mutationFn: async () => {
      const review = await request<Review & { targetVersion: number }>(
        `/projects/${projectId}/tasks/${taskId}/acceptance-review`,
      );
      return request(`/projects/${projectId}/tasks/${taskId}/acceptances`, {
        method: "POST",
        body: JSON.stringify({
          reviewId: review.reviewId,
          reviewHash: review.reviewHash,
          expectedVersion: review.targetVersion,
          decision: "accept",
        }),
        idempotencyKey: crypto.randomUUID(),
      });
    },
    onSuccess: invalidate,
  });
  if (!task) return null;
  return (
    <Card className="judex-thread-card">
      <Card.Content>
        <strong>{task.title}</strong>
        <small>
          {task.status} · v{task.version}
        </small>
        <div className="judex-inline-form">
          {task.status === "ready" && (
            <Button variant="secondary" isPending={start.isPending} onClick={() => start.mutate()}>
              {t("wpStart")}
            </Button>
          )}
          {(task.status === "working" || task.status === "rework" || task.status === "ready") && (
            <Button variant="secondary" isPending={deliver.isPending} onClick={() => deliver.mutate()}>
              {t("wpDeliver")}
            </Button>
          )}
          {task.status === "delivered" && (
            <Button variant="primary" isPending={accept.isPending} onClick={() => accept.mutate()}>
              {t("wpAccept")}
            </Button>
          )}
        </div>
        {(start.isError || deliver.isError || accept.isError) && (
          <UIWarning role="alert">
            {errorText(locale, start.error ?? deliver.error ?? accept.error)}
          </UIWarning>
        )}
      </Card.Content>
    </Card>
  );
}

function ProposalList({ projectId }: { projectId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [selected, setSelected] = useState<string | null>(null);

  const proposals = useQuery({
    queryKey: ["proposals", projectId],
    queryFn: () => request<{ items: Proposal[] }>(`/projects/${projectId}/proposals`),
    refetchInterval: 8000,
  });

  const create = useMutation({
    mutationFn: () =>
      request(`/projects/${projectId}/proposals`, {
        method: "POST",
        body: JSON.stringify({
          kind: "work_arrangement",
          changes: [{ operation: "create_plan", targetType: "plan", clientRef: "p", fields: { title: title.trim() } }],
        }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: () => {
      setTitle("");
      setCreating(false);
      client.invalidateQueries({ queryKey: ["proposals", projectId] });
    },
  });

  const invalidate = () => {
    client.invalidateQueries({ queryKey: ["proposals", projectId] });
    client.invalidateQueries({ queryKey: ["actions", projectId] });
  };

  return (
    <div className="judex-panel-stack">
      <div className="judex-workspace-section">
        <h3>{t("wpProposals")}</h3>
        <Button onClick={() => setCreating((v) => !v)}>
          <Plus size={15} />
          {t("wpNewProposal")}
        </Button>
      </div>
      {creating && (
        <form
          className="judex-inline-form"
          onSubmit={(event) => {
            event.preventDefault();
            if (title.trim()) create.mutate();
          }}
        >
          <Input aria-label={t("wpNewProposal")} value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
          <Button type="submit" isPending={create.isPending}>
            {create.isPending ? t("submitting") : t("pwCreate")}
          </Button>
          <Button variant="ghost" onClick={() => setCreating(false)}>
            <X size={15} />
          </Button>
          {create.isError && <span role="alert">{errorText(locale, create.error)}</span>}
        </form>
      )}
      {proposals.isPending ? (
        <p className="judex-workspace-status">{t("shellLoading")}</p>
      ) : (
        <ul className="judex-workspace-list">
          {(proposals.data?.items ?? []).map((proposal) => (
            <li key={proposal.id} className="judex-workspace-item">
              <Button variant={selected === proposal.id ? "secondary" : "ghost"} onClick={() => setSelected(proposal.id)}>
                {proposal.kind}
              </Button>
              <small>{proposal.status}</small>
            </li>
          ))}
          {(proposals.data?.items ?? []).length === 0 && <li className="judex-workspace-status">{t("wpNoProposals")}</li>}
        </ul>
      )}
      {selected && <ProposalDetail projectId={projectId} proposalId={selected} onChanged={invalidate} />}
    </div>
  );
}

function ProposalDetail({ projectId, proposalId, onChanged }: { projectId: string; proposalId: string; onChanged: () => void }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [reason, setReason] = useState("");
  const review = useQuery({
    queryKey: ["proposalReview", projectId, proposalId],
    queryFn: () => request<Review>(`/projects/${projectId}/proposals/${proposalId}/review`),
    enabled: !!proposalId,
    // 冻结审阅必须与当前状态一致：pending 期间持续保鲜。
    refetchInterval: (query) => (query.state.data?.status === "pending" ? 2000 : false),
  });
  const proposals = useQuery({
    queryKey: ["proposals", projectId],
    queryFn: () => request<{ items: Proposal[] }>(`/projects/${projectId}/proposals`),
  });
  const proposal = proposals.data?.items?.find((item) => item.id === proposalId);

  const submit = useMutation({
    mutationFn: () =>
      request(`/projects/${projectId}/proposals/${proposalId}/submit`, {
        method: "POST",
        body: JSON.stringify({ expectedVersion: proposal?.version ?? 1 }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: () => {
      // 冻结后的审阅（reviewId/hash/slots）必须重读，否则决定用空哈希被拒。
      client.invalidateQueries({ queryKey: ["proposalReview", projectId, proposalId] });
      onChanged();
    },
  });
  const decide = useMutation({
    mutationFn: (decision: "approve" | "reject") =>
      request(`/projects/${projectId}/proposals/${proposalId}/decisions`, {
        method: "POST",
        body: JSON.stringify({
          reviewId: review.data?.reviewId,
          reviewHash: review.data?.reviewHash,
          expectedVersion: proposal?.version ?? 1,
          decision,
          slotIds: (review.data?.slots ?? []).filter((s) => s.state === "pending").map((s) => s.id),
          reason,
        }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: onChanged,
  });

  if (proposals.isPending || review.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  return (
    <Card className="judex-thread-card">
      <Card.Content>
        <div className="judex-panel-stack">
          <strong>
            {proposal?.kind} · {proposal?.status}
          </strong>
          {review.data?.slots?.map((slot) => (
            <small key={slot.id}>
              {slot.displayName}
              {slot.positionName ? `（${slot.positionName}）` : ""} — {slot.state}
            </small>
          ))}
          {review.data?.deadlineAt && <small>deadline: {new Date(review.data.deadlineAt).toLocaleString()}</small>}
          {proposal?.status === "draft" && (
            <Button variant="secondary" isPending={submit.isPending} onClick={() => submit.mutate()}>
              {t("wpSubmitProposal")}
            </Button>
          )}
          {proposal?.status === "pending" && (
            <div className="judex-inline-form" data-ready={!!review.data?.reviewHash}>
              <Input
                aria-label={t("wpReason")}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={t("wpReason")}
              />
              <Button
                variant="primary"
                isPending={decide.isPending}
                isDisabled={!review.data?.reviewHash || !review.data?.slots?.length}
                onClick={() => decide.mutate("approve")}
              >
                {t("wpApprove")}
              </Button>
              <Button
                variant="danger"
                isPending={decide.isPending}
                isDisabled={!review.data?.reviewHash || !review.data?.slots?.length}
                onClick={() => {
                  if (reason.trim()) decide.mutate("reject");
                }}
              >
                {t("wpReject")}
              </Button>
            </div>
          )}
          {(submit.isError || decide.isError) && (
            <UIWarning role="alert">{errorText(locale, submit.error ?? decide.error)}</UIWarning>
          )}
        </div>
      </Card.Content>
    </Card>
  );
}
