import test from "node:test";
import assert from "node:assert/strict";
import {
  fitSplit,
  readSplit,
  DEFAULT_SPLIT,
} from "../../web/src/features/chat/splitLayout.ts";
const limits = { left: 200, center: 360, right: 320, divider: 8 };
test("invalid preferences fall back and wide layouts preserve intended proportions", () => {
  for (const value of [
    null,
    {},
    { left: NaN, right: 0.3 },
    { left: -1, right: 0.2 },
    { left: "0.2", right: 0.3 },
  ])
    assert.deepEqual(readSplit(value), DEFAULT_SPLIT);
  const s = fitSplit(1920, { left: 0.2, right: 0.3 }, limits, true);
  assert.equal(s.left, 384);
  assert.equal(s.right, 576);
  assert.equal(s.left + s.center + s.right + 16, 1920);
});
test("constrained panes share available space without clipping the center and collapsing preserves preferences", () => {
  const r = { left: 0.45, right: 0.45 };
  const s = fitSplit(1120, r, limits, true);
  assert.ok(s.left >= 200 && s.right >= 320 && s.center >= 359.9);
  const closed = fitSplit(1120, r, limits, false);
  assert.equal(closed.right, 0);
  assert.ok(closed.center >= 360);
  assert.deepEqual(r, { left: 0.45, right: 0.45 });
});
