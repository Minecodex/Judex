import test from "node:test";
import assert from "node:assert/strict";
import { EVENT_INVALIDATION_ROOTS, invalidationRoots } from "../../web/src/lib/api/eventMap.ts";
import {
  closeTab,
  openTab,
  parseTabKey,
  tabKey,
} from "../../web/src/features/workspace/tabs.ts";

test("sse invalidation covers every known project event type", () => {
  for (const type of Object.keys(EVENT_INVALIDATION_ROOTS)) {
    assert.ok(invalidationRoots(type).length > 0, `${type} must invalidate something`);
  }
});

test("sse invalidation: message events refresh messages/topics/runs; unknown events invalidate nothing", () => {
  assert.deepEqual(invalidationRoots("message.committed"), [
    "messages",
    "topics",
    "runs",
    "actions",
  ]);
  assert.ok(invalidationRoots("proposal.changed").includes("proposals"));
  assert.ok(invalidationRoots("handoff.changed").includes("handoffs"));
  assert.deepEqual(invalidationRoots("brand.new.event"), []);
  assert.deepEqual(invalidationRoots(""), []);
});

test("tab keys round-trip for home/topic/handoff and unknown keys fall back home", () => {
  assert.equal(tabKey({ kind: "home" }), "home");
  assert.equal(tabKey({ kind: "topic", id: "t1" }), "topic:t1");
  assert.equal(tabKey({ kind: "handoff", id: "h1" }), "handoff:h1");
  assert.deepEqual(parseTabKey("home"), { kind: "home" });
  assert.deepEqual(parseTabKey("topic:t1"), { kind: "topic", id: "t1" });
  assert.deepEqual(parseTabKey("handoff:h1"), { kind: "handoff", id: "h1" });
  for (const bad of [null, "", "topic", "topic:", "workspace:t1", "handoff:"]) {
    assert.deepEqual(parseTabKey(bad), { kind: "home" }, `${bad} → home`);
  }
});

test("openTab reuses existing tabs and activates; closeTab falls back to the neighbour", () => {
  const home = { tabs: [{ kind: "home" }], active: { kind: "home" } };
  const one = openTab(home, { kind: "topic", id: "t1" });
  assert.equal(one.tabs.length, 2);
  assert.deepEqual(one.active, { kind: "topic", id: "t1" });
  const again = openTab(one, { kind: "topic", id: "t1" });
  assert.equal(again.tabs.length, 2, "no duplicate tabs");
  const two = openTab(again, { kind: "handoff", id: "h1" });
  assert.equal(two.tabs.length, 3);
  // 关闭当前活动标签 → 回退到前一个标签
  const closed = closeTab(two, "handoff:h1");
  assert.equal(closed.tabs.length, 2);
  assert.deepEqual(closed.active, { kind: "topic", id: "t1" });
  // 首页标签不可关闭：集合与活动状态都保持不变
  const keptHome = closeTab(closed, "home");
  assert.equal(keptHome.tabs.length, 2);
  assert.deepEqual(keptHome.active, { kind: "topic", id: "t1" });
  // 关闭当前活动标签后活动回退到前一个，集合只剩首页
  const closedActive = closeTab(closed, "topic:t1");
  assert.equal(closedActive.tabs.length, 1);
  assert.deepEqual(closedActive.active, { kind: "home" });
});
