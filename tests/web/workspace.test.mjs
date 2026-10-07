import test from "node:test";
import assert from "node:assert/strict";
import { EVENT_INVALIDATION_ROOTS, invalidationRoots } from "../../web/src/lib/api/eventMap.ts";
import {
  closeTab,
  openTab,
  routeTab,
  tabKey,
} from "../../web/src/features/chat/conversationTabs.ts";

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

test("tab keys encode home/topic/handoff and routeTab reads route", () => {
  assert.equal(tabKey({ kind: "home" }), "home");
  assert.equal(tabKey({ kind: "topic", id: "t1" }), "topic:t1");
  assert.equal(tabKey({ kind: "handoff", id: "h1" }), "handoff:h1");
  const base = { design: "studio", projectId: "p", view: "home" };
  assert.deepEqual(routeTab({ ...base, view: "topic", id: "t1" }), {
    kind: "topic",
    id: "t1",
  });
  assert.deepEqual(routeTab({ ...base, view: "handoff", id: "h1" }), {
    kind: "handoff",
    id: "h1",
  });
  assert.deepEqual(routeTab({ ...base, conversation: "handoff:h1" }), {
    kind: "handoff",
    id: "h1",
  });
  assert.deepEqual(routeTab({ ...base, conversation: "t1" }), {
    kind: "topic",
    id: "t1",
  });
  assert.deepEqual(routeTab(base), { kind: "home" });
});

test("openTab dedupes and appends; closeTab falls back to the neighbour, home is not closable", () => {
  const home = { kind: "home" };
  let tabs = openTab([home], { kind: "topic", id: "t1" });
  assert.equal(tabs.length, 2);
  tabs = openTab(tabs, { kind: "topic", id: "t1" });
  assert.equal(tabs.length, 2, "no duplicate tabs");
  tabs = openTab(tabs, { kind: "handoff", id: "h1" });
  assert.equal(tabs.length, 3);
  // 关闭当前活动标签 → 活动回退到前一个
  const closed = closeTab(tabs, "handoff:h1", "handoff:h1");
  assert.equal(closed.tabs.length, 2);
  assert.equal(closed.active, "topic:t1");
  // 首页标签不可关闭：集合与活动都保持不变
  const keptHome = closeTab(closed.tabs, "home", closed.active);
  assert.equal(keptHome.tabs.length, 2);
  assert.equal(keptHome.active, "topic:t1");
  // 关闭当前活动标签后只剩首页
  const closedActive = closeTab(closed.tabs, "topic:t1", closed.active);
  assert.equal(closedActive.tabs.length, 1);
  assert.equal(closedActive.active, "home");
});
