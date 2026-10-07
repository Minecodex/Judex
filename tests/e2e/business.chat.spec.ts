// 统一工作区（ChatWorkspace 树）生产 E2E：注册→项目→进入统一工作区→
// 新建议题→发送消息（幂等提交）→消息可见→会话标签管理→右栏功能面板。
// SSE 事件流以"外部建议题→侧边栏自动出现"验证（替代旧的 ws-live 指示灯）。
import { expect, test } from "@playwright/test";

const stamp = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);

test.describe.configure({ mode: "serial" });

async function registerAndEnterWorkspace(page: import("@playwright/test").Page, name: string) {
  await page.goto("/register");
  await page.getByLabel(/名称|Name/).fill(name);
  await page.getByLabel(/邮箱|Email/).fill(`${name}-${stamp()}@judex.test`);
  await page.getByLabel(/密码|Password/, { exact: false }).first().fill(`password-${name}-11`);
  await page.getByLabel(/确认密码|Confirm/).fill(`password-${name}-11`);
  await page.getByRole("button", { name: /创建账号|Create account/ }).click();
  await expect(page.getByTestId("workspace-new-project")).toBeVisible({ timeout: 10000 });
  await page.getByTestId("workspace-new-project").click();
  await page.getByLabel(/新建项目|New project/).fill(`统一工作区-${name}`);
  await page.locator("form button[type=submit]").first().click();
  // 创建成功后进入统一工作区：项目切换器 + 右栏启动器 + 主会场会话
  await expect(page.getByTestId("project-switcher")).toBeVisible({ timeout: 20000 });
  await expect(page.getByTestId("workspace-launcher")).toBeVisible();
  await expect(page.getByTestId("chat-thread")).toBeVisible();
}

async function createTopicViaApi(page: import("@playwright/test").Page, title: string) {
  const session = (await (await page.request.get("/api/v1/auth/session")).json()).data;
  const projects = (await (await page.request.get("/api/v1/projects")).json()).data.items;
  const project = projects[0];
  await page.request.post(`/api/v1/projects/${project.id}/topics`, {
    headers: { "X-CSRF-Token": session.csrfToken, "Idempotency-Key": crypto.randomUUID() },
    data: { title, initialMessage: "来自外部的第一条消息" },
  });
}

test("W01 统一工作区布局与概览", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w01");
  await expect(page.getByTestId("chat-thread")).toBeVisible();
  // 右栏启动器提供功能入口（任务路线/共享资料/需要我决定/交接信箱/待办概览）
  await expect(page.getByTestId("work-nav-plans")).toBeVisible();
  // 账户入口在左下角
  await expect(page.getByTestId("account-menu-button")).toBeVisible();
  // SSE 项目事件流：外部创建议题后，侧边栏无需刷新自动出现
  await createTopicViaApi(page, "外部议题-w01");
  await expect(page.getByRole("button", { name: "外部议题-w01", exact: true })).toBeVisible({ timeout: 20000 });
  // 任务路线面板可以打开（含新建计划入口）
  await page.getByTestId("work-nav-plans").click();
  await expect(page.getByRole("button", { name: /开启一个计划|Begin a plan/ })).toBeVisible();
  await ctx.close();
});

test("W02 新建议题并发送消息（统一提交幂等）", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w02");
  await page.getByTestId("new-discussion").click();
  await page.getByTestId("discussion-title").fill("接口稳定性讨论");
  await page.getByTestId("create-discussion").click();
  // 议题创建后在会话标签中打开
  await expect(page.getByTestId("work-discussion-input")).toBeVisible({ timeout: 10000 });
  await expect(page.locator('[data-testid^="tab-topic:"]').first()).toBeVisible();
  await page.getByTestId("work-discussion-input").fill("下单接口幂等缺失，请评估影响。");
  await page.getByTestId("send-work-message").click();
  await expect(page.getByTestId("chat-thread")).toContainText("下单接口幂等缺失，请评估影响。", { timeout: 10000 });
  // 会话标签管理：议题标签打开后可关闭，回到主会场
  const tab = page.locator('[data-testid^="tab-topic:"]').first();
  await expect(tab).toBeVisible();
  await tab.hover();
  await page.getByRole("button", { name: /关闭标签 · 接口稳定性讨论/ }).click();
  await expect(page.locator('[data-testid^="tab-topic:"]')).toHaveCount(0);
  // 右栏功能面板切换：需要我决定
  await page.getByTestId("work-nav-decisions").first().click();
  await expect(page.getByText(/待办与验收|Decisions/).first()).toBeVisible();
  await ctx.close();
});
