// api 模式工作台：react-query 装配 WorkState，SSE 失效 apiWs 前缀；
// act 走 apiActions 的真实写操作（P2），未接的动作仍提示 shellUnavailable。
import { useRef } from "react";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "../auth/AuthProvider";
import { errorKey } from "../auth/errors";
import { useProjectEvents } from "../../lib/api/sse";
import { apiWorkspaceQueries, apiWsKey, assembleWorkState } from "./apiModel";
import { apiActions, type ApiActContext } from "./apiActions";
import { useWorkbenchBase } from "./storeBase";
import { allPeople, manage, member } from "./selectors";
import type { ActFn, ActResult, WorkStore } from "./storeTypes";

export type ApiWorkbench = {
  store: WorkStore | null;
  failed: boolean;
  retry: () => void;
};

export function useApiWorkbench(projectId: string): ApiWorkbench {
  const base = useWorkbenchBase();
  const { user } = useAuth();
  const q = apiWorkspaceQueries(projectId);
  const bootstrap = useQuery(q.bootstrap);
  const projects = useQuery(q.projects);
  const members = useQuery(q.members);
  const positions = useQuery(q.positions);
  const identities = useQuery(q.identities);
  const plans = useQuery(q.plans);
  const tasks = useQuery(q.tasks);
  const handoffs = useQuery(q.handoffs);
  const topics = useQuery(q.topics);
  const invitations = useQuery(q.invitations);
  const preferences = useQuery(q.preferences);
  const audit = useQuery(q.audit);
  const workflows = useQuery(q.workflows);
  const proposals = useQuery(q.proposals);
  const versions = useQueries({
    queries: (workflows.data ?? []).map((w) => q.workflowVersions(w.id)),
  });
  const taskDetails = useQueries({
    queries: (tasks.data ?? []).map((t) => q.taskDetail(t.id)),
  });
  const messages = useQueries({
    queries: (topics.data ?? []).map((t) => q.topicMessages(t.id)),
  });
  const pendingProposals = (proposals.data ?? []).filter(
    (p) => p.kind === "work_arrangement",
  );
  const reviews = useQueries({
    queries: pendingProposals.map((p) => q.proposalReview(p.id)),
  });
  const client = useQueryClient();
  useProjectEvents(projectId, bootstrap.data?.eventCursor, () => {
    void client.invalidateQueries({ queryKey: apiWsKey(projectId) });
  });
  const core = [
    bootstrap,
    projects,
    members,
    positions,
    identities,
    plans,
    tasks,
    handoffs,
    topics,
    invitations,
    preferences,
    audit,
    workflows,
    proposals,
  ];
  const ready =
    !!user &&
    [...core, ...versions, ...taskDetails, ...messages, ...reviews].every(
      (query) => query.isSuccess,
    );
  // 首次齐备后保持可用：SSE 失效引发的 refetch 或新增议题的子查询不再回到加载态。
  const loaded = useRef(false);
  loaded.current ||= ready;
  const failed = core.some((query) => query.isError);
  const retry = () =>
    void client.invalidateQueries({ queryKey: apiWsKey(projectId) });
  if (!loaded.current || !user || !bootstrap.data)
    return { store: null, failed, retry };
  const state = assembleWorkState(projectId, {
    bootstrap: bootstrap.data,
    projects: projects.data ?? [],
    members: members.data ?? [],
    positions: positions.data ?? [],
    identities: identities.data ?? [],
    plans: plans.data ?? [],
    tasks: (tasks.data ?? []).map((t, i) => taskDetails[i]?.data ?? t),
    handoffs: handoffs.data ?? [],
    topics: topics.data ?? [],
    topicMessages: Object.fromEntries(
      (topics.data ?? []).map((t, i) => [t.id, messages[i]?.data ?? []]),
    ),
    invitations: invitations.data ?? [],
    preferences: preferences.data ?? null,
    audit: audit.data ?? [],
    workflows: workflows.data ?? [],
    workflowVersions: Object.fromEntries(
      (workflows.data ?? []).map((w, i) => [w.id, versions[i]?.data ?? []]),
    ),
    proposals: proposals.data ?? [],
    proposalReviews: Object.fromEntries(
      pendingProposals.map((p, i) => [p.id, reviews[i]?.data]),
    ),
    currentUser: user.displayName,
  });
  const route = { ...base.route, projectId };
  const act: ActFn = async (name, payload, opts) => {
    const executor = apiActions[name] as
      | ((
          projectId: string,
          payload: never,
          ctx: ApiActContext,
        ) => Promise<ActResult>)
      | undefined;
    if (!executor) {
      base.setToast(base.t("shellUnavailable"));
      return { ok: false };
    }
    try {
      const result = await executor(projectId, payload as never, {
        state,
        userId: user.id,
        displayName: user.displayName,
      });
      void client.invalidateQueries({ queryKey: apiWsKey(projectId) });
      if (name === "createProject" && result.id) {
        location.assign("/?project=" + result.id);
        return result;
      }
      if (opts?.toast !== false) base.setToast(base.t("workSaved"));
      return result;
    } catch (error) {
      base.setToast(base.t(errorKey(error) ?? "errNetwork"));
      return { ok: false };
    }
  };
  const project =
    state.projects.find((p) => p.id === route.projectId) || state.projects[0];
  return {
    store: {
      state,
      route,
      go: base.go,
      locale: base.locale,
      setLocale: base.setLocale,
      theme: base.theme,
      setTheme: base.setTheme,
      toast: base.toast,
      setToast: base.setToast,
      storageError: false,
      text: base.text,
      t: base.t,
      act,
      switchPerson: () => {},
      reset: () => {},
      project,
      membership: member(state, project.id),
      management: manage(state, project.id),
      people: allPeople(state),
    },
    failed,
    retry,
  };
}
