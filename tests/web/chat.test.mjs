import test from "node:test";
import assert from "node:assert/strict";
import { seedWork } from "../../web/src/features/work/seed.ts";
import {
  submitProposal,
  decideProposal,
  discussTopic,
} from "../../web/src/features/chat/proposalModel.ts";
import { executionLayout } from "../../web/src/features/chat/executionLayout.ts";
import { pendingDecisionCount } from "../../web/src/features/chat/decisions.ts";
const proposal = {
  title: "New phase",
  goal: "A useful result",
  criteria: "Evidence checked",
  ownerSeatId: "lead",
  flowId: "delivery",
  tasks: [
    {
      title: "Build",
      seatId: "maker",
      reviewerSeatId: "lead",
      nodeId: "make",
      after: [],
    },
    {
      title: "Review",
      seatId: "receiver",
      reviewerSeatId: "lead",
      nodeId: "receive",
      after: [0],
    },
  ],
};
test("review and plan decisions stay discoverable, and an identity transfer invalidates the old vote target", () => {
  let s = seedWork();
  s.currentUser = "林然";
  assert.equal(pendingDecisionCount(s, "leaf"), 3);
  s = submitProposal(s, "labels", proposal).state;
  const p = s.proposals[0];
  s.seats.find((v) => v.id === "maker").person = "江澄";
  assert.equal(decideProposal(s, p.id, p.revision, true).error, "stale");
});
test("dependency display retains hard requirements and unresolved handoffs instead of presenting them as independent", () => {
  const s = seedWork();
  const tasks = structuredClone(s.tasks.slice(0, 2));
  tasks[1].requirements = [
    {
      id: "one",
      kind: "task",
      ref: tasks[0].id,
      hard: false,
      at: "accept",
      label: { zh: "", en: "" },
    },
    {
      id: "two",
      kind: "task",
      ref: tasks[0].id,
      hard: true,
      at: "start",
      label: { zh: "", en: "" },
    },
  ];
  const g = executionLayout(tasks, []);
  assert.equal(g.edges.length, 1);
  assert.equal(g.edges[0].hard, true);
  assert.equal(g.edges[0].at, "both");
  const p = s.tasks.find((t) => t.id === "package");
  assert.ok(
    executionLayout([p], []).external.get("package").includes("first-review"),
  );
});
test("discussion is read-only for formal work; submitted proposal needs all named humans exactly once", () => {
  let s = seedWork();
  const tasks = structuredClone(s.tasks);
  s = discussTopic(s, "labels").state;
  assert.deepEqual(s.tasks, tasks);
  s = submitProposal(s, "labels", proposal).state;
  const p = s.proposals[0];
  assert.deepEqual(p.approvers, ["林然", "顾言", "周宁"]);
  assert.equal(s.plans.length, 3);
  assert.equal(
    decideProposal({ ...s, currentUser: "沈言" }, p.id, 1, true).error,
    "permission",
  );
  for (const name of p.approvers) {
    s.currentUser = name;
    s = decideProposal(s, p.id, 1, true).state;
  }
  assert.equal(s.proposals[0].status, "approved");
  assert.equal(s.plans.length, 4);
  assert.equal(s.tasks.length, tasks.length + 2);
  const work = s.tasks.slice(-2);
  assert.equal(work[1].requirements[0].ref, work[0].id);
  assert.equal(decideProposal(s, p.id, 1, true).error, "stale");
});
test("rejection preserves work, revisions reset votes, flow and identity changes reject stale approval", () => {
  let s = submitProposal(seedWork(), "labels", proposal).state;
  const id = s.proposals[0].id;
  s = decideProposal(s, id, 1, false, "Needs revision").state;
  assert.equal(s.plans.length, 3);
  s = submitProposal(s, "labels", proposal, id).state;
  assert.equal(s.proposals[1].revision, 2);
  assert.deepEqual(s.proposals[1].votes, {});
  s.flows[0].version++;
  assert.equal(decideProposal(s, s.proposals[1].id, 2, true).error, "stale");
});
test("proposal validates node-bound identities and predecessor order", () => {
  const s = seedWork();
  const wrong = structuredClone(proposal);
  wrong.tasks[0].seatId = "receiver";
  assert.equal(submitProposal(s, "labels", wrong).error, "scope");
  const cycle = structuredClone(proposal);
  cycle.tasks[0].after = [1];
  assert.equal(submitProposal(s, "labels", cycle).error, "scope");
});
test("execution layout follows prerequisites, not parentage; handoff joins and external dependencies stay explicit", () => {
  const s = seedWork();
  const graph = executionLayout(
    s.tasks.filter((t) => t.projectId === "leaf"),
    s.handoffs,
  );
  assert.equal(
    graph.positions.get("empty-state").rank,
    graph.positions.get("build").rank,
  );
  assert.ok(graph.positions.get("package").x > graph.positions.get("build").x);
  assert.ok(graph.positions.get("package").x > graph.positions.get("guide").x);
  const one = executionLayout(
    [s.tasks.find((t) => t.id === "package")],
    s.handoffs,
  );
  assert.ok(one.external.get("package").includes("build"));
  const tasks = structuredClone(s.tasks.slice(0, 2));
  tasks[0].requirements = [
    {
      id: "a",
      kind: "task",
      ref: tasks[1].id,
      hard: true,
      label: { zh: "", en: "" },
    },
  ];
  tasks[1].requirements = [
    {
      id: "b",
      kind: "task",
      ref: tasks[0].id,
      hard: true,
      label: { zh: "", en: "" },
    },
  ];
  assert.equal(executionLayout(tasks, []).invalid.length, 2);
});
