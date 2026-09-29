import {useCollection,LoadMore} from "../../lib/api/collections";
import { useAuth } from "../auth/AuthProvider";
import type { Identity } from "../projects/TeamSettings";
import { ResearchPanel } from "./ResearchPanel";
import { PlanDecisions, ExecutionGraph, HandoffEditor, type WorkItem } from "./LifecyclePanels";
import { WorkChangeForm } from "./WorkChangeForm";
import { ArrangementForm } from "./ArrangementForm";
import { ReviewDetails, type Change, type EvidenceReview } from "./ReviewDetails";
import { UIWarning, FormField, UISelect, UIOption, UICheckbox } from "../../components/ui/FormControls";
import { Button, Card, Input } from "@heroui/react";
import { Plus, X } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { usePreferences } from "../../stores/preferences";
import { translate, type Key } from "../../i18n";
import { request } from "../../lib/api/client";
import { errorText } from "../projects/ProjectWorkspace";

type Plan = { id: string; title: string; status: string; version: number; taskStats: { total: number; accepted: number } };
type Task = WorkItem & { participants: {identityId:string}[]; id: string; title: string; status: string; version: number; planId: string | null };
type Proposal = { id: string; kind: string; status: string; version: number };
type Review = {
  version: number;
  reviewId: string;
  reviewHash: string;
  status: string;
  slots: { id: string; displayName: string; state: string; positionName: string | null; authorityType: string; authorityId: string; bindingVersion: number; canDecide: boolean; canDelegate: boolean }[];
  deadlineAt: string | null;
  changes: Change[];
};

// WorkPanel（P3-10）：计划/任务/提案/待办的真实 API 面板。所有命令携带
// expectedVersion 与幂等键；批准/验收不做乐观更新。
export function WorkPanel({ projectId, onOpenHandoff }: { projectId: string; onOpenHandoff?: (id: string) => void }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [creatingPlan, setCreatingPlan] = useState(false);
  const [planTitle, setPlanTitle] = useState("");
  const [selectedPlan, setSelectedPlan] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<string | null>(null);

  const plans=useCollection<Plan>(["plans",projectId],`/projects/${projectId}/plans`);
  const actions = useQuery({
    queryKey: ["actions", projectId],
    queryFn: () => request<{ items: { kind: string; objectType: string; objectId: string; summary: string }[] }>(
      `/me/actions?projectId=${projectId}`,
    ),
    refetchInterval: 8000,
  });
  const tasks=useCollection<Task>(["tasks",projectId,selectedPlan??""],`/projects/${projectId}/tasks${selectedPlan?`?planId=${selectedPlan}`:""}`);
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
      <LoadMore query={plans} label={t("wpPlans")} />
      {selectedPlan ? <TaskList projectId={projectId} planId={selectedPlan} onOpen={setSelectedTask} /> : <ul className="judex-workspace-list">{(tasks.data?.items ?? []).map((task) => <li key={task.id}><Button variant="ghost" onPress={() => setSelectedTask(task.id)}>{task.title} · {task.status}</Button></li>)}</ul>}
      {!selectedPlan&&<LoadMore query={tasks} label={t("paTask")} />}
      {selectedPlan && <PlanDecisions key={selectedPlan} projectId={projectId} planId={selectedPlan} />}
      {selectedTask && <TaskDetail key={selectedTask} projectId={projectId} taskId={selectedTask} />}
      <ExecutionGraph projectId={projectId} onTask={setSelectedTask} />
      <ResearchPanel projectId={projectId} />
      <HandoffEditor projectId={projectId} onOpen={onOpenHandoff} />
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
  const tasks=useCollection<Task>(["tasks",projectId,planId],`/projects/${projectId}/tasks?planId=${planId}`);
  if (tasks.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  return (
    <><ul className="judex-workspace-list">
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
    </ul><LoadMore query={tasks} /></>
  );
}

function TaskDetail({ projectId, taskId }: { projectId: string; taskId: string }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const detail = useQuery({ queryKey: ["task", projectId, taskId], queryFn: () => request<Task>(`/projects/${projectId}/tasks/${taskId}`) });
  const task = detail.data;
  const { user } = useAuth(); const [identity, setIdentity] = useState(""); const [materials, setMaterials] = useState<string[]>([]);
  const identities = useQuery({queryKey:["identities",projectId],queryFn:()=>request<{items:Identity[]}>(`/projects/${projectId}/identities`)});
  const availableMaterials = useQuery({queryKey:["materials",projectId],queryFn:()=>request<{items:{id:string;title:string;currentVersionId:string}[]}>(`/projects/${projectId}/materials`)});
  const held=(identities.data?.items??[]).filter((i)=>i.currentBinding?.userId===user?.id && task?.participants?.some((p)=>p.identityId===i.id));
  const actingIdentity=held.length===1?held[0].id:identity;

  const [changing, setChanging] = useState(false);
  const [reason, setReason] = useState("");
  const invalidate = () => {
    client.invalidateQueries({ queryKey: ["tasks", projectId] });
    client.invalidateQueries({ queryKey: ["task", projectId, taskId] });
    client.invalidateQueries({ queryKey: ["actions", projectId] });
    for(const key of ["plans","plan","executionMap"]) void client.invalidateQueries({queryKey:[key,projectId]});
  };
  const start = useMutation({
    mutationFn: () =>
      request(`/projects/${projectId}/tasks/${taskId}/start`, {
        method: "POST",
        body: JSON.stringify({ expectedVersion: task?.version ?? 1, identityId: actingIdentity }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: invalidate,
  });
  const deliver = useMutation({
    mutationFn: (kind: "progress" | "delivery") =>
      request(`/projects/${projectId}/tasks/${taskId}/reports`, {
        method: "POST",
        body: JSON.stringify({ kind, text: reportText, expectedTaskVersion: task?.version ?? 1, identityId: actingIdentity, materialVersionIds: materials }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: invalidate,
  });
  const [acceptanceReview, setAcceptanceReview] = useState<EvidenceReview | null>(null);
  const [reportText, setReportText] = useState("");
  const openReview = useMutation({ mutationFn: () => request<EvidenceReview>(`/projects/${projectId}/tasks/${taskId}/acceptance-review`), onSuccess: setAcceptanceReview });
  const accept = useMutation({
    mutationFn: async (decision: "accept" | "reject") => {
      const review = acceptanceReview;
      if (!review) throw new Error("Review required");
      return request(`/projects/${projectId}/tasks/${taskId}/acceptances`, {
        method: "POST",
        body: JSON.stringify({
          reviewId: review.reviewId,
          reviewHash: review.reviewHash,
          expectedVersion: review.targetVersion,
          decision, reason,
        }),
        idempotencyKey: crypto.randomUUID(),
      });
    },
    onSuccess: () => { setAcceptanceReview(null); invalidate(); },
  });
  const reopen = useMutation({ mutationFn: () => request(`/projects/${projectId}/tasks/${taskId}/reopens`, { method: "POST", idempotencyKey: crypto.randomUUID(), body: JSON.stringify({ expectedVersion: task?.version, acceptanceId: task?.latestAcceptanceId, reason }) }), onSuccess: invalidate });
  if (!task) return null;
  return (
    <Card className="judex-thread-card">
      <Card.Content>
        <strong>{task.title}</strong>
        <small>
          {task.status} · v{task.version}
        </small>
        <FormField label={t("paParticipants")}><UISelect value={actingIdentity} onChange={(e)=>setIdentity(e.target.value)}><UIOption value="">{t("paChoose")}</UIOption>{held.map((i)=><UIOption key={i.id} value={i.id}>{i.positionName}</UIOption>)}</UISelect></FormField>
        {availableMaterials.data?.items?.map((m)=><UICheckbox key={m.id} checked={materials.includes(m.currentVersionId)} onChange={()=>setMaterials((values)=>values.includes(m.currentVersionId)?values.filter((v)=>v!==m.currentVersionId):[...values,m.currentVersionId])}>{m.title}</UICheckbox>)}
        <Input aria-label={t("accessReport")} value={reportText} onChange={(e) => setReportText(e.target.value)} />
        <div className="judex-inline-form">
          {task.status === "ready" && (
            <Button variant="secondary" isPending={start.isPending} isDisabled={!actingIdentity} onClick={() => start.mutate()}>
              {t("wpStart")}
            </Button>
          )}
          {(task.status === "working" || task.status === "rework" || task.status === "ready") && (
            <Button variant="secondary" isPending={deliver.isPending} isDisabled={!actingIdentity || !reportText.trim()} onClick={() => deliver.mutate("delivery")}>
              {t("wpDeliver")}
            </Button>
          )}
          {(task.status === "working" || task.status === "rework") && <Button isDisabled={!actingIdentity || !reportText.trim()} isPending={deliver.isPending} onPress={()=>deliver.mutate("progress")}>{t("lcProgress")}</Button>}
          {task.status === "delivered" && (
            <Button variant="primary" isPending={accept.isPending} onClick={() => openReview.mutate()}>
              {t("wpAccept")}
            </Button>
          )}
        </div>
        {acceptanceReview && <>
          <ReviewDetails evidence={acceptanceReview} />
          <Button isPending={accept.isPending} isDisabled={acceptanceReview.blockers?.some((b) => b.phase === "accept" || b.phase === "both")} onPress={() => accept.mutate("accept")}>{t("accessConfirm")}</Button>
          <Input aria-label={t("paReason")} value={reason} onChange={(e) => setReason(e.target.value)} />
          <Button variant="danger" isDisabled={!reason.trim()} isPending={accept.isPending} onPress={() => accept.mutate("reject")}>{t("wpReject")}</Button>
          <Button variant="ghost" onPress={() => setAcceptanceReview(null)}>{t("accessCancel")}</Button>
        </>}
        {openReview.isError && <UIWarning>{openReview.error.message}</UIWarning>}
        {task.status === "accepted" && <>
          <Input aria-label={t("paReason")} value={reason} onChange={(e) => setReason(e.target.value)} />
          <Button isDisabled={!reason.trim()} isPending={reopen.isPending} onPress={() => reopen.mutate()}>{t("lcReopen")}</Button>
        </>}
        {reopen.isError && <UIWarning>{reopen.error.message}</UIWarning>}
        {task.status !== "accepted" && task.status !== "cancelled" && <Button variant="ghost" onPress={() => setChanging(!changing)}>{t("lcChange")}</Button>}
        {changing && <WorkChangeForm projectId={projectId} target={task} kind="task" />}
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
  const [selected, setSelected] = useState<string | null>(null);

  const proposals=useCollection<Proposal>(["proposals",projectId],`/projects/${projectId}/proposals`);
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
      {creating && <ArrangementForm projectId={projectId} onCreated={() => setCreating(false)} />}
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
      <LoadMore query={proposals} />
      {selected && <ProposalDetail projectId={projectId} proposalId={selected} onChanged={invalidate} />}
    </div>
  );
}

function ProposalDetail({ projectId, proposalId, onChanged }: { projectId: string; proposalId: string; onChanged: () => void }) {
  const { locale } = usePreferences();
  const t = (key: Key) => translate(locale, key);
  const client = useQueryClient();
  const [reason, setReason] = useState("");
  const [delegated,setDelegated]=useState<string[]>([]);
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
        body: JSON.stringify({ expectedVersion: review.data?.version ?? proposal?.version ?? 1, draftHash: review.data?.reviewHash }),
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
          expectedVersion: review.data?.version ?? proposal?.version ?? 1,
          decision,
          slotIds: (review.data?.slots ?? []).filter((s) => s.canDecide).map((s) => s.id),
          actingBindingVersions: (review.data?.slots ?? []).filter((s) => s.canDecide && s.authorityType === "identity").map((s) => ({ identityId: s.authorityId, bindingVersion: s.bindingVersion })),
          reason,
        }),
        idempotencyKey: crypto.randomUUID(),
      }),
    onSuccess: onChanged,
  });

  const delegate=useMutation({mutationFn:()=>request(`/projects/${projectId}/proposals/${proposalId}/delegate-decisions`,{method:"POST",body:JSON.stringify({reviewId:review.data?.reviewId,reviewHash:review.data?.reviewHash,slotIds:delegated,reason})}),onSuccess:()=>{setDelegated([]);onChanged();}});
  if (proposals.isPending || review.isPending) return <p className="judex-workspace-status">{t("shellLoading")}</p>;
  return (
    <Card className="judex-thread-card">
      <Card.Content>
        <div className="judex-panel-stack">
          <strong>
            {proposal?.kind} · {proposal?.status}
          </strong>
          <ReviewDetails changes={review.data?.changes} />
          {review.data?.slots?.map((slot) => (
            <small key={slot.id}>
              {slot.displayName}
              {slot.positionName ? `（${slot.positionName}）` : ""} — {slot.state}
            </small>
          ))}
          {review.data?.deadlineAt && <small>deadline: {new Date(review.data.deadlineAt).toLocaleString()}</small>}
          {review.data?.status === "draft" && (
            <Button variant="secondary" isPending={submit.isPending} onClick={() => submit.mutate()}>
              {t("wpSubmitProposal")}
            </Button>
          )}
          {review.data?.status === "pending" && (
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
                isDisabled={!review.data?.reviewHash || !review.data?.slots?.some((s) => s.canDecide)}
                onClick={() => decide.mutate("approve")}
              >
                {t("wpApprove")}
              </Button>
              <Button
                variant="danger"
                isPending={decide.isPending}
                isDisabled={!review.data?.reviewHash || !review.data?.slots?.some((s) => s.canDecide)}
                onClick={() => {
                  if (reason.trim()) decide.mutate("reject");
                }}
              >
                {t("wpReject")}
              </Button>
            </div>
          )}
          {review.data?.status==="pending" && review.data.slots.some((s)=>s.canDelegate)&&<section className="judex-panel-stack">
            <strong>{t("weDelegateSelect")}</strong>{review.data.slots.filter((s)=>s.canDelegate).map((s)=><UICheckbox key={s.id} checked={delegated.includes(s.id)} onChange={()=>setDelegated((values)=>values.includes(s.id)?values.filter((id)=>id!==s.id):[...values,s.id])}>{s.displayName} · {s.positionName}</UICheckbox>)}
            <Button variant="secondary" isDisabled={!reason.trim()||!delegated.length} isPending={delegate.isPending} onPress={()=>delegate.mutate()}>{t("weDelegateConfirm")}</Button>
          </section>}
          {delegate.isError&&<UIWarning>{delegate.error.message}</UIWarning>}
          {(submit.isError || decide.isError) && (
            <UIWarning role="alert">{errorText(locale, submit.error ?? decide.error)}</UIWarning>
          )}
        </div>
      </Card.Content>
    </Card>
  );
}
