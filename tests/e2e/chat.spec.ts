import {openTool,openSettings,selectPerson,toggleTheme,toggleLanguage} from './workspace-helpers';
import { test, expect, type Page } from "@playwright/test";
const state = async (page: Page) => {
  // 演示工作区为懒加载分块：seed 在挂载后写入，直接等其就绪
  //（team/flows 视图重定向到全屏设置页，账户按钮保持隐藏）。
  await expect
    .poll(
      async () =>
        (await page.evaluate(() => localStorage.getItem("judex.web.preview.v1"))) !==
        null,
      { timeout: 10000 },
    )
    .toBe(true);
  return page.evaluate(() =>
    JSON.parse(localStorage.getItem("judex.web.preview.v1")!),
  );
};
test("chat remains in place while exploring settings and execution order, with bilingual dark mode", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/?project=leaf&view=topic&item=labels");
  await page.getByTestId("work-discussion-input").fill("保留这个草稿");
  await openTool(page,"plans");
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "保留这个草稿",
  );
  const build = page.getByTestId("execution-task-build"),
    sub = page.getByTestId("execution-task-empty-state"),
    pack = page.getByTestId("execution-task-package");
  await expect(build).toHaveAttribute("data-rank", "0");
  await expect(sub).toHaveAttribute("data-rank", "0");
  await expect(pack).toHaveAttribute("data-rank", "1");
  await page.getByTestId("execution-task-build").click();
  await expect(page.locator(".judex-chat-panel-body")).toContainText(
    "让任务创建和列表顺畅可用",
  );
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "保留这个草稿",
  );
  await page.reload();
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "保留这个草稿",
  );
  for (const view of ["resources", "team", "flows"]) {
    await openTool(page,view);
    if(await page.getByTestId("settings-page").isVisible())await page.getByTestId("settings-back").click();
    await expect(page.getByTestId("chat-thread")).toContainText(
      "标签能否留到下一步",
    );
  }
  await toggleTheme(page);
  await toggleLanguage(page);
  await page.setViewportSize({ width: 1120, height: 900 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: test.info().outputPath("chat-dark-en.png"),
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
test("new project can configure workflows, roles, an existing member and personal preferences without leaving chat", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "新建项目", exact: true }).click();
  await page.getByTestId("project-project-name").fill("山间小店");
  await page
    .getByLabel("项目目标", { exact: true })
    .fill("一起完成一个更轻松的小店");
  await page.getByTestId("project-create-project").click();
  await expect(page.getByTestId("workspace-launcher")).toBeVisible();
  await openTool(page,"flows");
  await page.getByRole("button", { name: "新增协作流程", exact: true }).click();
  await page.getByRole("dialog").getByLabel("这件事叫什么").fill("小店日常");
  await page
    .getByRole("dialog")
    .getByLabel("告诉伙伴希望怎样协作")
    .fill("准备方案，再进行人工验收");
  await page
    .getByRole("dialog")
    .getByLabel("环节名称（每行一个，生成顺序示例图）")
    .fill("准备方案\n验收成果");
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "确认发布新版本" })
    .click();
  await openTool(page,"team");
  await page.getByTestId("new-position").click();
  await page.getByTestId("position-name").fill("小店伙伴");
  await page.getByTestId("position-prompt").fill("准备方案并说明成果依据");
  await page.getByTestId("save-position").click();
  await page.getByRole("button", { name: "分配职位", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("checkbox", { name: "小店伙伴" })
    .press("Space");
  await page.getByTestId("assign-existing-position").click();
  await openTool(page,"preferences");
  await page.getByTestId("next-personal-prompt").fill("先给我简洁结论");
  await page.getByTestId("next-save-preference").click();
  await page.getByTestId("settings-back").click();
  await page
    .getByTestId("work-discussion-input")
    .fill("我们从第一个小目标开始");
  await page.getByTestId("send-work-message").click();
  await expect(page.getByTestId("chat-thread")).toContainText(
    "本轮已整理职责意见",
  );
  const s = await state(page);
  const p = s.projects.at(-1);
  expect(s.flows.find((f: any) => f.projectId === p.id).nodes).toHaveLength(2);
  expect(s.preferences.find((v: any) => v.projectId === p.id).prompt).toBe(
    "先给我简洁结论",
  );
  await page.screenshot({
    path: test.info().outputPath("new-project.png"),
    fullPage: true,
  });
});
test("conversation proposal freezes recipients and creates dependency-linked work only after all votes", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  const before = await state(page);
  await page.getByTestId("chat-discuss").click();
  await expect(page.getByTestId("chat-thread")).toContainText(
    "本轮已整理职责意见",
  );
  await page.getByTestId("chat-propose").click();
  await page.getByTestId("proposal-title").fill("从聊天到执行");
  await page.getByTestId("proposal-criteria").fill("交付证据经过人工核对");
  await page
    .getByRole("dialog")
    .locator("fieldset")
    .nth(1)
    .locator('[data-slot="checkbox-content"]')
    .first()
    .click();
  await expect(page.getByRole("dialog").locator("fieldset").nth(1).getByRole("checkbox").first()).toBeChecked();
  await page.getByTestId("send-proposal").click();
  let s = await state(page);
  const p = s.proposals.at(-1);
  expect(s.plans).toEqual(before.plans);
  expect(s.tasks).toEqual(before.tasks);
  for (const name of p.approvers) {
    await selectPerson(page,name);
    await page.goto("/?project=leaf&view=topic&item=labels");
    await page.getByTestId("approve-proposal-" + p.id).click();
  }
  s = await state(page);
  expect(s.proposals.at(-1).status).toBe("approved");
  expect(s.plans.length).toBe(before.plans.length + 1);
  expect(s.tasks.slice(-3)[1].requirements).toHaveLength(1);
  await page
    .getByRole("button", { name: "打开已生效的计划", exact: true })
    .click();
  await expect(page.getByTestId("execution-map")).toBeVisible();
});
