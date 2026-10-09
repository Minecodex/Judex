import test from "node:test";
import assert from "node:assert/strict";
import { seedWork } from "../../web/src/features/work/seed.ts";
import {
  topicMessage,
  reportTask,
} from "../../web/src/features/work/actions.ts";
import {
  setDiscussionLimit,
  roundLimit,
  discussTopic,
  latestDiscussionRun,
  discussWorkSubmission,
} from "../../web/src/features/chat/discussionPolicy.ts";
test("only project managers can set an integer limit, and project settings are independent", () => {
  const seed = seedWork();
  assert.equal(roundLimit(seed.projects[0]), 3);
  assert.equal(setDiscussionLimit(seed, "leaf", 2).error, "permission");
  seed.currentUser = "林然";
  for (const value of [0, -1, 1.5, 101, NaN])
    assert.equal(setDiscussionLimit(seed, "leaf", value).error, "required");
  const s = setDiscussionLimit(seed, "leaf", 2).state;
  assert.equal(roundLimit(s.projects[0]), 2);
  assert.equal(roundLimit(s.projects[1]), 3);
});
test("submission budget is frozen; repeated AI analysis cannot reset it; new material starts a fresh budget", () => {
  let s = seedWork();
  s.currentUser = "林然";
  s = setDiscussionLimit(s, "leaf", 2).state;
  s = topicMessage(s, "labels", "第一份提交").state;
  assert.equal(latestDiscussionRun(s, "labels").rounds, 1);
  const first = latestDiscussionRun(s, "labels").id;
  s = discussTopic(s, "labels").state;
  assert.equal(latestDiscussionRun(s, "labels").status, "limit");
  assert.equal(discussTopic(s, "labels").error, "discussionLimit");
  s = setDiscussionLimit(s, "leaf", 4).state;
  assert.equal(latestDiscussionRun(s, "labels").maxRounds, 2);
  assert.equal(discussTopic(s, "labels").error, "discussionLimit");
  s = topicMessage(s, "labels", "补充材料", [
    {
      id: "material1",
      name: "补充.md",
      text: "新依据",
      author: s.currentUser,
      at: Date.now(),
    },
  ]).state;
  const last = latestDiscussionRun(s, "labels");
  assert.equal(last.rounds, 1);
  assert.equal(last.maxRounds, 4);
  assert.equal(last.trigger, "material");
  assert.notEqual(last.id, first);
  assert.equal(s.topics[0].discussionRuns[0].rounds, 2);
});
test("old closed flags never lock chat, and reaching a limit does not approve or finish work", () => {
  let s = seedWork();
  s.currentUser = "林然";
  s.topics[0].closed = true;
  s = setDiscussionLimit(s, "leaf", 1).state;
  const before = structuredClone(s);
  s = topicMessage(s, "labels", "新的意见").state;
  assert.equal(latestDiscussionRun(s, "labels").rounds, 1);
  assert.deepEqual(s.tasks, before.tasks);
  assert.deepEqual(s.plans, before.plans);
  assert.deepEqual(s.handoffs, before.handoffs);
  s = topicMessage(s, "labels", "仍然可以继续发言").state;
  assert.equal(s.topics[0].discussionRuns.length, 2);
});
test("work reporting creates one task analysis without task conversations, replay is idempotent", () => {
 let s=seedWork();s.currentUser="顾言";
 s=reportTask(s,"build","本地进展与验证结果",[]).state;
 const activity=s.taskActivities.at(-1),count=s.topics.length;
 assert.equal(activity.taskId,"build");
 assert.equal(activity.analysis.state,"completed");
 assert.equal(s.topics.some(t=>t.title.zh.startsWith("工作上报：")),false);
 const replay=discussWorkSubmission(s,"build",activity.id,"本地进展与验证结果",[]).state;
 assert.deepEqual(replay,s);
 assert.equal(replay.topics.length,count);
});
