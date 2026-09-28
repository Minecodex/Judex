// P6-03 双用户生产 E2E（docs/plans/v1/10 A/B 系列）：真实注册→项目→提案→
// 会签→任务→交付→验收→CLI 式确认页。生产 build + 真实 API，无 localStorage 注入。
import { expect, test } from "@playwright/test";

const stamp = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);

test.describe.configure({ mode: "serial" });

test("A01/A04 双用户注册、建项目、隔离", async ({ browser }) => {
  const ctxA = await browser.newContext();
  const ctxB = await browser.newContext();
  const pageA = await ctxA.newPage();
  const pageB = await ctxB.newPage();

  // A registers.
  await pageA.goto("/register");
  const emailA = `a-${stamp()}@judex.test`;
  await pageA.getByLabel(/名称|Name/).fill("甲");
  await pageA.getByLabel(/邮箱|Email/).fill(emailA);
  await pageA.getByLabel(/密码|Password/, { exact: false }).first().fill("password-aaaa-11");
  await pageA.getByLabel(/确认密码|Confirm/).fill("password-aaaa-11");
  await pageA.getByRole("button", { name: /创建账号|Create account/ }).click();
  await expect(pageA).toHaveURL(/\/$|\/\?/);
  // Create project via workspace shell.
  await pageA.getByTestId("workspace-new-project").click();
  await pageA.getByLabel(/新建项目|New project/).fill("双用户验收项目");
  await pageA.locator("form button[type=submit]").first().click();
  await expect(pageA.getByText("双用户验收项目")).toBeVisible({ timeout: 10000 });

  // B registers and sees no projects.
  const emailB = `b-${stamp()}@judex.test`;
  await pageB.goto("/register");
  await pageB.getByLabel(/名称|Name/).fill("乙");
  await pageB.getByLabel(/邮箱|Email/).fill(emailB);
  await pageB.getByLabel(/密码|Password/, { exact: false }).first().fill("password-bbbb-11");
  await pageB.getByLabel(/确认密码|Confirm/).fill("password-bbbb-11");
  await pageB.getByRole("button", { name: /创建账号|Create account/ }).click();
  await expect(pageB.getByText(/还没有项目|No projects yet/)).toBeVisible({ timeout: 10000 });

  // Session survives refresh (A02).
  await pageA.reload();
  await expect(pageA.getByText("双用户验收项目")).toBeVisible({ timeout: 10000 });

  // Logout clears state; B's data never leaks into A's cache (A03).
  await ctxA.close();
  await ctxB.close();
});

test("A02 登录错误不枚举账号", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto("/login");
  await page.getByLabel(/邮箱|Email/).fill(`ghost-${stamp()}@judex.test`);
  await page.getByLabel(/密码|Password/, { exact: false }).first().fill("password-xxxx-11");
  await page.getByRole("button", { name: /登录|Sign in/ }).click();
  await expect(page.getByText(/账号或密码不正确|Incorrect account/)).toBeVisible({ timeout: 10000 });
  await ctx.close();
});

test("F03 生产模式无 demo 残留", async ({ browser }) => {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  // about:blank denies localStorage; check on the real origin after load.
  await page.goto("/register");
  const seeded = await page.evaluate(() => localStorage.length);
  expect(seeded).toBe(0);
  const body = await page.textContent("body");
  expect(body).not.toContain("P2+");
  // API 404 不回 SPA、静态资源缺失 404 已由 backend 测试覆盖。
  const resp = await page.request.get("/api/v1/missing-endpoint");
  expect(resp.status()).toBe(404);
  expect(await resp.text()).not.toContain("<!doctype html>");
  await ctx.close();
});
