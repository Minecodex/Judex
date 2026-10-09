import test from "node:test";
import assert from "node:assert/strict";
import { seedWork } from "../../web/src/features/work/seed.ts";
import {
  sourceDecision,
  reviseSource,
  sendSource,
  taskAction,
  reportTask,
  planAction,
  createWork,
  decideDraft,
  topicMessage,
  publishFlow,
  refreshHandoff,
  invitePerson,
  acceptInvite,
  replaceSeat,
  personalPrompt,
  savePosition,
  remindPending,
} from "../../web/src/features/work/actions.ts";
import { createDiscussion, proposeHandoff } from "../../web/src/features/work/composition.ts";
import {
  blockers,
  needsReview,
  planTasks,
  planReviewKey,
} from "../../web/src/features/work/selectors.ts";
const ok = (result) => {
  assert.ok(result.state, result.error);
  return result.state;
};
const as = (state, name) => ({ ...state, currentUser: name });
const task = (s, id) => s.tasks.find((t) => t.id === id);
test("acceptance decisions stay bound to the actual reviewed task and plan evidence", () => {
  const initial = seedWork(),
    reviewedRevision = task(initial, "guide").revision;
  const changed = ok(
    reportTask(as(initial, "夏禾"), "guide", "补充后的新版说明", []),
  );
  assert.equal(
    taskAction(as(changed, "林然"), "guide", "accept", "", reviewedRevision)
      .error,
    "stale",
  );
  let s = as(seedWork(), "林然");
  s.tasks = s.tasks.map((t) =>
    t.planId === "leaf-first" ? { ...t, status: "accepted" } : t,
  );
  const reviewedPlan = planReviewKey(s, s.plans[0]);
  const updated = structuredClone(s);
  task(updated, "guide").revision++;
  assert.equal(
    planAction(updated, "leaf-first", "accept", reviewedPlan).error,
    "stale",
  );
});
const source = (s, id) => s.handoffs[0].sources.find((v) => v.id === id);
function receiveAll(state = seedWork()) {
  return ok(
    sourceDecision(
      ok(
        sourceDecision(
          as(state, "周宁"),
          "first-review",
          "source-api",
          1,
          "accepted",
        ),
      ),
      "first-review",
      "source-guide",
      1,
      "accepted",
    ),
  );
}
test("plans, tasks and discussions are distinct; references never own or duplicate a task", () => {
  const s = seedWork();
  assert.equal(planTasks(s, "leaf-first").length, 5);
  assert.equal(planTasks(s, "leaf-next").length, 0);
  assert.deepEqual(s.plans.find((p) => p.id === "leaf-next").referenceTaskIds, [
    "build",
  ]);
  assert.equal(task(s, "research").planId, null);
  assert.ok(s.topics.every((t) => !("work" in t)));
});
test("receipt is explicit, authorized and never final task or plan acceptance", () => {
  const s = seedWork();
  assert.equal(
    sourceDecision(as(s, "沈言"), "first-review", "source-api", 1, "accepted")
      .error,
    "permission",
  );
  assert.equal(
    sourceDecision(s, "first-review", "source-api", 9, "accepted").error,
    "stale",
  );
  const next = ok(
    sourceDecision(s, "first-review", "source-api", 1, "accepted"),
  );
  assert.equal(source(next, "source-api").status, "accepted");
  assert.equal(source(s, "source-api").status, "pending");
  assert.deepEqual(next.tasks, s.tasks);
  assert.deepEqual(next.plans, s.plans);
});
test("partial refusal, revision and resend preserve other sources and actual actors", () => {
  let s = ok(
    sourceDecision(seedWork(), "first-review", "source-api", 1, "accepted"),
  );
  assert.equal(
    sourceDecision(s, "first-review", "source-guide", 1, "rejected", "").error,
    "required",
  );
  s = ok(
    sourceDecision(
      s,
      "first-review",
      "source-guide",
      1,
      "rejected",
      "请补充空状态截图",
    ),
  );
  const accepted = structuredClone(source(s, "source-api"));
  assert.equal(task(s, "guide").status, "rework");
  assert.equal(task(s, "build").status, "delivered");
  assert.equal(
    reviseSource(s, "first-review", "source-guide", 1, "补好了", []).error,
    "permission",
  );
  s = ok(
    reviseSource(
      as(s, "夏禾"),
      "first-review",
      "source-guide",
      1,
      "补齐空状态与操作说明",
      [],
    ),
  );
  assert.equal(source(s, "source-guide").status, "draft");
  assert.deepEqual(source(s, "source-api"), accepted);
  s = ok(sendSource(s, "first-review", "source-guide", 2));
  assert.equal(source(s, "source-guide").sentBy, "夏禾");
  assert.equal(
    sourceDecision(as(s, "周宁"), "first-review", "source-guide", 1, "accepted")
      .error,
    "stale",
  );
  s = ok(
    sourceDecision(
      as(s, "周宁"),
      "first-review",
      "source-guide",
      2,
      "accepted",
    ),
  );
  assert.equal(source(s, "source-guide").decidedBy, "周宁");
  assert.ok(
    s.handoffs[0].history.some((h) => h.source.reason === "请补充空状态截图"),
  );
});
test("hard conditions distinguish start prerequisites from output required at acceptance", () => {
  const s = seedWork();
  assert.equal(taskAction(s, "package", "start").error, "blocked");
  const received = receiveAll(s);
  assert.equal(
    blockers(received, task(received, "package"), "start").length,
    0,
  );
  assert.equal(
    blockers(received, task(received, "package"), "accept").length,
    1,
  );
  const started = ok(taskAction(received, "package", "start"));
  const delivered = ok(
    reportTask(started, "package", "本地体验走查完成，附验证说明", []),
  );
  assert.equal(
    taskAction(as(delivered, "沈言"), "package", "accept").error,
    "permission",
  );
  const accepted = ok(
    taskAction(
      as(delivered, "林然"),
      "package",
      "accept",
      "",
      task(delivered, "package").revision,
    ),
  );
  assert.equal(task(accepted, "package").status, "accepted");
  assert.equal(accepted.plans[0].status, "active");
});
test("only the plan owner accepts the whole plan; reopening a task does not rewrite that decision", () => {
  let s = receiveAll();
  assert.equal(
    planAction(
      as(s, "林然"),
      "leaf-first",
      "accept",
      planReviewKey(s, s.plans[0]),
    ).error,
    "blocked",
  );
  s.tasks = s.tasks.map((t) =>
    t.planId === "leaf-first" ? { ...t, status: "accepted" } : t,
  );
  assert.equal(
    planAction(as(s, "沈言"), "leaf-first", "accept").error,
    "permission",
  );
  s = ok(
    planAction(
      as(s, "林然"),
      "leaf-first",
      "accept",
      planReviewKey(s, s.plans[0]),
    ),
  );
  const oldAt = s.plans[0].acceptedAt;
  s = ok(taskAction(s, "build", "reopen", "需要复核数据边界"));
  assert.equal(s.plans[0].status, "accepted");
  assert.equal(s.plans[0].acceptedAt, oldAt);
  assert.equal(needsReview(s, s.plans[0]), true);
  assert.ok(
    s.events.some(
      (e) => e.targetId === "leaf-first" && e.text.zh.includes("整体验收"),
    ),
  );
});
test("new workflow context keeps unaffected cards and invalidates materially affected pending cards", () => {
  const original = seedWork();
  let s = ok(
    publishFlow(
      as(original, "沈言"),
      "delivery",
      1,
      "措辞优化，不改变交付内容",
      false,
    ),
  );
  assert.equal(s.flows[0].version, 2);
  assert.equal(s.handoffs[0].stale, false);
  s = ok(
    sourceDecision(as(s, "周宁"), "first-review", "source-api", 1, "accepted"),
  );
  s = ok(
    publishFlow(
      as(s, "沈言"),
      "delivery",
      2,
      "待接收交付需要说明访问范围",
      true,
    ),
  );
  assert.equal(
    sourceDecision(as(s, "周宁"), "first-review", "source-guide", 1, "accepted")
      .error,
    "stale",
  );
  s = ok(refreshHandoff(as(s, "周宁"), "first-review"));
  assert.equal(source(s, "source-api").status, "accepted");
  assert.equal(source(s, "source-guide").status, "draft");
  assert.equal(s.handoffs[0].flowVersion, 3);
  assert.equal(
    sendSource(as(s, "周宁"), "first-review", "source-guide", 2).error,
    "permission",
  );
  s = ok(sendSource(as(s, "夏禾"), "first-review", "source-guide", 2));
  assert.equal(source(s, "source-guide").status, "pending");
});
test("discussion messages and linking never mutate work progress", () => {
  const s = seedWork();
  let n = ok(topicMessage(s, "labels", "我同意，也全部完成了"));
  n = ok(
    createDiscussion(
      n,
      "leaf",
      "另一个问题",
      ["leaf-first", "leaf-next"],
      ["build", "guide"],
    ),
  );
  assert.deepEqual(n.tasks, s.tasks);
  assert.deepEqual(n.plans, s.plans);
  assert.deepEqual(n.handoffs, s.handoffs);
  assert.equal(
    createDiscussion(s, "leaf", "错误关联", ["wild-launch"], []).error,
    "scope",
  );
  assert.equal(
    topicMessage(as(s, "赵可"), "labels", "未加入时写入").error,
    "permission",
  );
});
test("work drafts keep one owning plan and require an authorized arrangement decision", () => {
  const s = seedWork(),
    draft = {
      kind: "task",
      title: "核对文案",
      description: "一份复核记录",
      criteria: "出处清楚",
      seatId: "writer",
      flowId: "delivery",
      planId: null,
    };
  const n = ok(createWork(s, "leaf", draft)),
    id = n.tasks.at(-1).id;
  assert.equal(task(n, id).status, "draft");
  assert.equal(task(n, id).planId, null);
  assert.equal(decideDraft(n, id).error, "permission");
  assert.equal(task(ok(decideDraft(as(n, "林然"), id)), id).status, "ready");
  assert.equal(
    createWork(s, "leaf", { ...draft, planId: "wild-launch" }).error,
    "scope",
  );
});
test("the same template supports many people, while private preferences stay with their person", () => {
  let s = seedWork(),
    before = s.seats.filter((v) => v.positionId === "build-role").length;
  assert.equal(before, 2);
  s = ok(acceptInvite(as(s, "赵可"), "invite-zhao"));
  assert.equal(
    s.seats.filter((v) => v.positionId === "build-role").length,
    before + 1,
  );
  assert.equal(s.seats.filter((v) => v.person === "赵可").length, 2);
  s = ok(personalPrompt(s, "leaf", "build-role", 0, "赵可的私人偏好"));
  const seat = s.seats.find(
    (v) => v.person === "赵可" && v.positionId === "build-role",
  );
  const workBefore = structuredClone(s.tasks);
  s = ok(replaceSeat(as(s, "林然"), seat.id, "赵可", "周宁"));
  assert.deepEqual(s.tasks, workBefore);
  assert.equal(
    s.preferences.find((v) => v.person === "周宁"),
    undefined,
  );
  assert.equal(
    s.preferences.find((v) => v.person === "赵可").prompt,
    "赵可的私人偏好",
  );
  assert.equal(s.seats.find((v) => v.id === seat.id).person, "周宁");
});
test("management configuration is separate from work approval and requires a real node", () => {
  const s = seedWork(),
    p = {
      name: "审阅",
      prompt: "核对证据",
      flowId: "delivery",
      nodeId: "receive",
    };
  assert.equal(savePosition(s, "leaf", p).error, "permission");
  assert.ok(savePosition(as(s, "沈言"), "leaf", p).state);
  assert.equal(
    savePosition(as(s, "沈言"), "leaf", { ...p, nodeId: "missing" }).error,
    "scope",
  );
  assert.equal(
    invitePerson(s, "leaf", "新同事", ["build-role"]).error,
    "permission",
  );
  assert.ok(
    invitePerson(as(s, "沈言"), "leaf", "新同事", ["build-role"]).state,
  );
  assert.equal(
    taskAction(as(s, "沈言"), "build", "accept").error,
    "permission",
  );
});
test("both handoff kinds start as drafts and cannot silently reassign the next task", () => {
  const s = as(seedWork(), "顾言");
  let n = ok(proposeHandoff(s, "leaf", "build", "lead", ["build"], "stage"));
  assert.equal(n.handoffs.at(-1).kind, "stage");
  assert.equal(n.handoffs.at(-1).sources[0].status, "draft");
  assert.deepEqual(n.tasks, s.tasks);
  n = ok(
    proposeHandoff(
      s,
      "leaf",
      "package",
      "receiver",
      ["build", "guide"],
      "dependency",
    ),
  );
  assert.equal(n.handoffs.at(-1).sources.length, 2);
  assert.equal(
    proposeHandoff(s, "leaf", "package", "writer", ["build"], "dependency")
      .error,
    "scope",
  );
});
test("reminders never manufacture receipt and an accepted outcome cannot be edited by its sender", () => {
  const original = seedWork();
  const s = ok(remindPending(as(original, "沈言"), "leaf"));
  assert.deepEqual(s.tasks, original.tasks);
  assert.deepEqual(s.handoffs, original.handoffs);
  let n = ok(
    taskAction(
      as(original, "林然"),
      "guide",
      "accept",
      "",
      task(original, "guide").revision,
    ),
  );
  n = ok(
    sourceDecision(
      as(n, "周宁"),
      "first-review",
      "source-guide",
      1,
      "rejected",
      "发现缺少依据",
    ),
  );
  assert.equal(task(n, "guide").status, "accepted");
  assert.equal(
    reviseSource(
      as(n, "夏禾"),
      "first-review",
      "source-guide",
      1,
      "直接修改已经验收的成果",
      [],
    ).error,
    "accepted",
  );
});
