import {chooseValue} from "./workspace-helpers";
import {openTool,openSettings,selectPerson,toggleTheme,toggleLanguage} from './workspace-helpers';
import { test, expect, type Page } from "@playwright/test";
test("an open acceptance dialog cannot approve evidence changed by a later local report", async ({
  page,
  context,
}) => {
  await open(page);
  await person(page, "林然");
  await open(page, "task", "guide");
  await page.getByTestId("accept-task").click();
  await expect(page.getByTestId("confirm-final-acceptance")).toBeEnabled();
  const other = await context.newPage();
  await open(other);
  await person(other, "夏禾");
  await open(other, "task", "guide");
  await other.getByTestId("report-task").click();
  await other
    .getByTestId("report-body")
    .fill("这是在审阅开始后提交的新版本，需要重新核对。");
  await other.getByTestId("submit-report").click();
  await person(other, "林然");
  await expect(page.getByTestId("confirm-final-acceptance")).toBeDisabled();
  await expect(page.getByRole("dialog")).toContainText("内容或状态已经变化");
  expect(
    (await state(page)).tasks.find((t: any) => t.id === "guide").status,
  ).toBe("delivered");
  await other.close();
});
const key = "judex.web.preview.v1";
const state = (page: Page) =>
  page.evaluate((k) => JSON.parse(localStorage.getItem(k)!), key);
const open = (page: Page, view = "home", id = "", design = "studio") =>
  page.goto(
    "/?design=" +
      design +
      "&project=leaf&view=" +
      view +
      (id ? "&item=" + id : ""),
  );
async function person(page: Page, name: string) {
  await selectPerson(page,name);
  await expect.poll(async () => (await state(page)).currentUser).toBe(name);
}
async function report(page: Page, taskId: string, actor: string) {
  await person(page, actor);
  await open(page, "task", taskId);
  await page.getByTestId("report-task").click();
  await page.getByTestId("report-body").fill("已在本地完成并记录验证结果。");
  await page.getByTestId("submit-report").click();
}
async function acceptTask(page: Page, taskId: string) {
  await open(page, "task", taskId);
  await page.getByTestId("accept-task").click();
  await page.getByTestId("confirm-final-acceptance").click();
}

for (const design of ["studio"]) {
  test(
    design +
      ": partial rejection -> local revision -> sender confirmation -> explicit receipt",
    async ({ page }) => {
      await open(
        page,
        design === "mail" ? "home" : "handoff",
        design === "mail" ? "" : "first-review",
        design,
      );
      await page.getByTestId("receive-source-api").click();
      const accepted = (await state(page)).handoffs[0].sources[0];
      await page.getByTestId("reject-source-guide").click();
      await page.getByTestId("work-reason").fill("请补充空状态截图。");
      await page.getByTestId("confirm-work-reason").click();
      await expect(
        page.getByTestId("contribution-source-guide"),
      ).toHaveAttribute("data-status", "rejected");
      await person(page, "夏禾");
      await open(page, "handoff", "first-review", design);
      await page.getByTestId("revise-source-guide").click();
      await page
        .getByTestId("report-body")
        .fill("已经补齐空状态截图和说明，供重新核对。");
      await page.getByTestId("work-files").setInputFiles({
        name: "补充说明.md",
        mimeType: "text/markdown",
        buffer: Buffer.from("空状态、首次创建、失败恢复均有说明。"),
      });
      await page.getByTestId("submit-report").click();
      await expect(
        page.getByTestId("contribution-source-guide"),
      ).toHaveAttribute("data-status", "draft");
      await page.getByTestId("send-source-guide").click();
      await expect(
        page.getByTestId("contribution-source-guide"),
      ).toHaveAttribute("data-status", "pending");
      await person(page, "周宁");
      await open(page, "handoff", "first-review", design);
      await page.getByTestId("receive-source-guide").click();
      const data = await state(page);
      expect(data.handoffs[0].sources[0]).toEqual(accepted);
      expect(data.handoffs[0].sources[1].revision).toBe(2);
      expect(data.handoffs[0].sources[1].status).toBe("accepted");
      expect(data.tasks.find((t: any) => t.id === "guide").status).toBe(
        "delivered",
      );
      expect(data.plans[0].status).toBe("active");
    },
  );
}

test("hard prerequisites, task acceptance, plan owner acceptance and reopening form a complete path", async ({
  page,
}) => {
  await open(page, "task", "package");
  await expect(page.getByTestId("start-task")).toBeDisabled();
  await open(page, "handoff", "first-review");
  await page.getByTestId("receive-source-api").click();
  await page.getByTestId("receive-source-guide").click();
  await open(page, "task", "package");
  await expect(page.getByTestId("start-task")).toBeEnabled();
  await page.getByTestId("start-task").click();
  await report(page, "package", "周宁");
  await report(page, "empty-state", "江澄");
  await person(page, "沈言");
  await open(page, "task", "package");
  await expect(page.getByTestId("accept-task")).toHaveCount(0);
  await person(page, "林然");
  for (const id of ["build", "guide", "empty-state", "package"])
    await acceptTask(page, id);
  await open(page);
  await openTool(page,'overview');
  await expect(page.getByTestId('today-plan-review-leaf-first')).toBeVisible();
  await open(page, "plan", "leaf-first");
  await page.getByTestId("accept-plan").click();
  await page.getByTestId("confirm-final-acceptance").click();
  expect((await state(page)).plans[0].status).toBe("accepted");
  await open(page, "task", "build");
  await page.getByTestId("reopen-task").click();
  await page.getByTestId("work-reason").fill("发现分页边界需要复核");
  await page.getByTestId("confirm-work-reason").click();
  await open(page, "plan", "leaf-first");
  await expect(page.getByTestId("plan-reopened-warning")).toBeVisible();
  expect((await state(page)).plans[0].status).toBe("accepted");
});

test("new plan and direct task start as drafts; references do not duplicate ownership", async ({
  page,
}) => {
  await open(page, "plans");
  await page.getByRole("button", { name: "开启一个计划", exact: true }).click();
  await page.getByTestId("new-work-title").fill("下一轮体验改善");
  await page.getByTestId("new-work-description").fill("清楚地改善首次使用体验");
  await page.getByTestId("new-work-criteria").fill("有验证依据");
  await chooseValue(page,"new-work-seat","lead");
  await page.getByTestId("create-work-draft").click();
  expect((await state(page)).plans.at(-1).status).toBe("draft");
  await expect(page.getByTestId("activate-plan")).toHaveCount(0);
  const id = (await state(page)).plans.at(-1).id;
  await person(page, "林然");
  await open(page, "plan", id);
  await page.getByTestId("activate-plan").click();
  await open(page, "tasks");
  await page.getByRole("button", { name: "提出一项任务", exact: true }).click();
  await page.getByTestId("new-work-title").fill("确认一个临时问题");
  await page.getByTestId("new-work-description").fill("附上出处的结论");
  await page.getByTestId("new-work-criteria").fill("资料可追溯");
  await chooseValue(page,"new-work-plan","");
  await page.getByTestId("create-work-draft").click();
  await page.getByTestId("approve-task-draft").click();
  expect((await state(page)).tasks.at(-1).planId).toBeNull();
  await open(page, "plan", "leaf-next");
  await expect(page.getByText("引用成果 · 不重复计入所属任务")).toBeVisible();
  expect(
    (await state(page)).tasks.filter((t: any) => t.planId === "leaf-next"),
  ).toHaveLength(0);
});

test("pure discussions link multiple plans and tasks without changing their progress", async ({
  page,
}) => {
  await open(page, "topics");
  const before = await state(page);
  await page.getByTestId("new-discussion").click();
  await page.getByTestId("discussion-title").fill("讨论下一阶段投入");
  await page.getByTestId("topic-plan-leaf-first").check();
  await page.getByTestId("topic-plan-leaf-next").check();
  await page.getByTestId("topic-task-build").check();
  await page.getByTestId("create-discussion").click();
  await page.getByTestId("work-discussion-input").fill("同意，可以全部完成");
  await page.getByTestId("send-work-message").click();
  const after = await state(page);
  expect(after.tasks).toEqual(before.tasks);
  expect(after.plans).toEqual(before.plans);
  expect(after.topics.at(-1).planIds).toEqual(["leaf-first", "leaf-next"]);
  await expect(page.getByTestId("work-discussion-input")).toBeEnabled();
});

test("a same-task handoff draft requires its sender and does not silently complete the task", async ({
  page,
}) => {
  await open(page);
  await person(page, "顾言");
  await open(page, "task", "build");
  await page.getByTestId("propose-handoff").click();
  await chooseValue(page,"handoff-kind","stage");
  await page.getByTestId("create-handoff-draft").click();
  const handoff = (await state(page)).handoffs.at(-1);
  expect(handoff.kind).toBe("stage");
  await page.getByTestId("send-" + handoff.sources[0].id).click();
  await person(page, "林然");
  await open(page, "handoff", handoff.id);
  await page.getByTestId("receive-" + handoff.sources[0].id).click();
  expect(
    (await state(page)).tasks.find((t: any) => t.id === "build").status,
  ).toBe("delivered");
});

test("the manager can publish new workflow context, and affected pending cards need reconfirmation", async ({
  page,
}) => {
  await open(page);
  await person(page, "沈言");
  await open(page, "flows");
  await expect(
    page.getByTestId("workflow-diagram").locator("svg"),
  ).toBeVisible();
  const diagramBox = await page
    .getByTestId("workflow-diagram")
    .locator("svg")
    .boundingBox();
  expect(diagramBox!.width).toBeGreaterThan(300);
  expect(diagramBox!.height).toBeGreaterThan(30);
  await page.getByTestId("workflow-input").fill("待接收交付补充其访问范围。");
  await page.getByTestId("prepare-flow-draft").click();
  await page.getByTestId("flow-material-change").check();
  await page.getByTestId("publish-flow").click();
  await person(page, "周宁");
  await open(page, "handoff", "first-review");
  await expect(page.getByTestId("handoff-stale")).toBeVisible();
  await expect(page.getByTestId("receive-source-api")).toHaveCount(0);
  await page.getByTestId("refresh-handoff").click();
  expect(
    (await state(page)).handoffs[0].sources.every(
      (s: any) => s.status === "draft",
    ),
  ).toBe(true);
  await person(page, "顾言");
  await open(page, "handoff", "first-review");
  await page.getByTestId("send-source-api").click();
  expect((await state(page)).handoffs[0].sources[1].status).toBe("draft");
});

test("position templates, invitation, personal preferences and replacement keep their boundaries", async ({
  page,
}) => {
  await open(page, "team");
  await expect(page.getByTestId("new-position")).toHaveCount(0);
  await person(page, "沈言");
  await open(page, "team");
  await page.getByTestId("new-position").click();
  await page.getByTestId("position-name").fill("研究伙伴");
  await page.getByTestId("position-prompt").fill("为结论附来源与适用范围");
  await chooseValue(page,"position-node","receive");
  await page.getByTestId("save-position").click();
  await page.getByTestId("next-invite").click();
  await page.getByTestId("next-invite-name").fill("陆青");
  await page.getByTestId("invite-position-build-role").check();
  await page.getByTestId("invite-position-content-role").check();
  await page.getByTestId("next-create-invite").click();
  await person(page, "陆青");
  await expect(page.getByTestId("work-nav-plans")).toHaveCount(0);
  await page.getByTestId("next-accept-invite").click();
  await page.getByTestId("next-personal-prompt").fill("陆青私有偏好");
  await page.getByTestId("next-save-preference").click();
  let data = await state(page);
  expect(
    data.seats.filter((s: any) => s.positionId === "build-role"),
  ).toHaveLength(3);
  const seat = data.seats.find(
    (s: any) => s.person === "陆青" && s.positionId === "build-role",
  );
  await person(page, "林然");
  await open(page, "team");
  await page
    .getByTestId("seat-" + seat.id)
    .getByRole("button", { name: "查看组合提示词", exact: true })
    .click();
  await expect(page.getByRole("dialog")).not.toContainText("陆青私有偏好");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "关闭", exact: true })
    .click();
  await page
    .getByTestId("seat-" + seat.id)
    .getByRole("button", { name: "替换关联人", exact: true })
    .click();
  await chooseValue(page,"replace-seat-person","周宁");
  await page.getByTestId("confirm-seat-replace").click();
  data = await state(page);
  expect(data.preferences.find((p: any) => p.person === "陆青").prompt).toBe(
    "陆青私有偏好",
  );
  expect(data.seats.find((s: any) => s.id === seat.id).person).toBe("周宁");
});

