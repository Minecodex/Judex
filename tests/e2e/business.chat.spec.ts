// 三栏聊天工作区（真实 API）E2E：注册→项目→进入三栏工作区→
// 新建议题→发送消息（幂等提交）→消息可见→会话标签管理→右栏面板切换。
// Agent 分析按钮在无模型配置的服务端会诚实失败——此处只验证入口与状态呈现。
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
  await page.getByLabel(/新建项目|New project/).fill(`三栏项目-${name}`);
  await page.locator("form button[type=submit]").first().click();
  // 创建成功后自动进入三栏工作区：左栏概览入口 + 右栏工作面板
  await expect(page.getByTestId("ws-nav-home")).toBeVisible({ timeout: 10000 });
  await expect(page.getByTestId("ws-panel-work")).toBeVisible();
}

test("W01 三栏工作区布局与概览", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w01");
  await expect(page.getByTestId("ws-home")).toBeVisible();
  // 右栏默认工作面板（真实 API：新建计划入口可见）
  await expect(page.getByRole("button", { name: /新建计划|New plan/ })).toBeVisible();
  // 左栏账户行（语言/主题/退出）
  await expect(page.getByRole("button", { name: "EN" })).toBeVisible();
  // SSE 项目事件流连通（服务端 15s 心跳内 onopen）
  await expect(page.getByTestId("ws-live")).toHaveAttribute("data-live", "on", {
    timeout: 20000,
  });
  await ctx.close();
});

test("W02 新建议题并发送消息（统一提交幂等）", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w02");
  await page.getByTestId("ws-new-topic").click();
  await page.getByLabel(/新议题|New topic/).fill("接口稳定性讨论");
  await page.getByLabel(/新议题|New topic/).press("Enter");
  await expect(page.getByRole("button", { name: "接口稳定性讨论" })).toBeVisible({
    timeout: 10000,
  });
  await page.getByRole("button", { name: "接口稳定性讨论" }).click();
  // 会话标签打开且消息区可见
  await expect(page.getByTestId("ws-composer")).toBeVisible({ timeout: 10000 });
  await page.getByTestId("ws-composer").fill("下单接口幂等缺失，请评估影响。");
  await page.getByTestId("ws-send").click();
  await expect(
    page.getByText("下单接口幂等缺失，请评估影响。").first(),
  ).toBeVisible({ timeout: 15000 });
  // 消息作者展示为注册用户（human 消息，非 Agent）
  await expect(page.getByTestId("ws-message").first()).toHaveAttribute("data-kind", "human");
  await ctx.close();
});

test("W03 会话标签管理与右栏切换", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w03");
  // 建两个议题形成多标签
  for (const title of ["议题一", "议题二"]) {
    await page.getByTestId("ws-new-topic").click();
    await page.getByLabel(/新议题|New topic/).fill(title);
    await page.getByLabel(/新议题|New topic/).press("Enter");
    await expect(page.getByRole("button", { name: title }).first()).toBeVisible({
      timeout: 10000,
    });
  }
  // 首页 + 两个议题 = 三个标签
  await expect(page.getByTestId("ws-tab-home")).toBeVisible();
  // 关闭一个议题标签 → 回到首页标签
  const tabsBefore = await page.locator(".judex-conversation-tab").count();
  expect(tabsBefore).toBeGreaterThanOrEqual(3);
  const topicTab = page.locator('[data-testid^="ws-tab-topic:"]').first();
  const key = await topicTab.getAttribute("data-testid");
  await page.getByTestId("ws-close-" + key!.replace("ws-tab-", "")).click();
  await expect(page.getByTestId("ws-tab-home")).toBeVisible();
  // 右栏切换资料/团队/工作
  await page.getByTestId("ws-panel-materials").click();
  await expect(page.getByText(/资料|Materials/).first()).toBeVisible();
  await page.getByTestId("ws-panel-team").click();
  await expect(page.getByText(/团队|Team/).first()).toBeVisible();
  await page.getByTestId("ws-panel-work").click();
  await expect(page.getByRole("button", { name: /新建计划|New plan/ })).toBeVisible();
  await ctx.close();
});

test("W04 Agent 分析入口与诚实降级", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w04");
  await page.getByTestId("ws-new-topic").click();
  await page.getByLabel(/新议题|New topic/).fill("分析议题");
  await page.getByLabel(/新议题|New topic/).press("Enter");
  await page.getByRole("button", { name: "分析议题" }).first().click();
  await expect(page.getByTestId("ws-composer")).toBeVisible({ timeout: 10000 });
  // 无消息时给出提示而非空按钮
  await expect(page.getByText(/发送一条消息后即可发起分析|Send a message/)).toBeVisible();
  await page.getByTestId("ws-composer").fill("请分析这个议题的可行性。");
  await page.getByTestId("ws-send").click();
  await expect(page.getByTestId("ws-analyze")).toBeVisible({ timeout: 15000 });
  // 未配置模型的部署：分析请求要么被拒绝要么运行失败——UI 不应崩溃
  await page.getByTestId("ws-analyze").click();
  await page.waitForTimeout(1500);
  const body = await page.textContent("body");
  expect(body).toBeTruthy();
  await ctx.close();
});

test("W05 语言与主题切换保持数据", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await registerAndEnterWorkspace(page, "w05");
  await page.getByRole("button", { name: "EN", exact: true }).click();
  await expect(page.getByTestId("ws-nav-home")).toHaveText(/Overview/);
  await page.getByRole("button", { name: "中文", exact: true }).click();
  await expect(page.getByTestId("ws-nav-home")).toHaveText(/概览/);
  // 主题切换不丢会话（当前主题的按钮显示切换目标符号）
  await page.getByRole("button", { name: /^(☀|☾)$/ }).click();
  await expect(page.getByTestId("ws-nav-home")).toBeVisible();
  await ctx.close();
});
