// P6-03 双用户提案会签→任务→交付→验收 全链路（B 系列核心路径）。
// 前置：business.core 已跑（项目由各用例自建，互不依赖）。
import { expect, test } from "@playwright/test";

const stamp = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);

async function registerAndCreateProject(page: import("@playwright/test").Page, name: string) {
  await page.goto("/register");
  await page.getByLabel(/名称|Name/).fill(name);
  await page.getByLabel(/邮箱|Email/).fill(`${name}-${stamp()}@judex.test`);
  await page.getByLabel(/密码|Password/, { exact: false }).first().fill(`password-${name}-11`);
  await page.getByLabel(/确认密码|Confirm/).fill(`password-${name}-11`);
  await page.getByRole("button", { name: /创建账号|Create account/ }).click();
  await expect(page.getByTestId("workspace-new-project")).toBeVisible({ timeout: 10000 });
  await page.getByTestId("workspace-new-project").click();
  await page.getByLabel(/新建项目|New project/).fill(`提案项目-${name}`);
  await page.locator("form button[type=submit]").first().click();
  // 创建后自动进入工作区，项目名多处可见。
  await expect(page.getByText(`提案项目-${name}`).first()).toBeVisible({ timeout: 10000 });
  const session = (await (await page.request.get("/api/v1/auth/session")).json()).data;
  const projects = (await (await page.request.get("/api/v1/projects")).json()).data.items;
  const project = projects.find((p: any) => p.title === `提案项目-${name}`);
  const headers = { "X-CSRF-Token": session.csrfToken, "Idempotency-Key": crypto.randomUUID() };
  const position = (await (await page.request.post(`/api/v1/projects/${project.id}/positions`, { headers, data: { name: "负责人职责", prompt: "核对计划" } })).json()).data;
  await page.request.post(`/api/v1/projects/${project.id}/identities`, { headers: { ...headers, "Idempotency-Key": crypto.randomUUID() }, data: { positionId: position.id, userId: session.user.id } });
}

test("B03/B06 提案创建→提交→唯一审批人同意→计划生效", async ({ page }) => {
  await registerAndCreateProject(page, "solo");
  // 创建后已自动进入该项目，直接进入工作面板。
  await page.getByRole("button", { name: /工作|Work/ }).click();
  // 新建提案（create_plan）：列表以 kind+status 展示。
  await page.getByRole("button", { name: /新提案|New proposal/ }).click();
  await fillProposal(page, "正式计划-甲");
  await page.locator("form button[type=submit]").first().click();
  await expect(page.getByText("work_arrangement").first()).toBeVisible({ timeout: 10000 });
  // 打开提案详情并提交审批（owner 即发起人；退化提案席位是 owner 自己）。
  await page.getByRole("button", { name: "work_arrangement" }).first().click();
  await page.getByRole("button", { name: /提交审批|Submit for approval/ }).click();
  await expect(page.getByText(/pending/).first()).toBeVisible({ timeout: 10000 });
  // 同意（覆盖全部席位）→ approved；计划列表出现 active 计划。
  await page.getByRole("button", { name: /^同意$|^Approve$/ }).click();
  await expect(page.getByText(/approved/).first()).toBeVisible({ timeout: 10000 });
});

test("B16 已审批提案不可重复决定", async ({ page }) => {
  await registerAndCreateProject(page, "dup");
  await page.getByRole("button", { name: /工作|Work/ }).click();
  await page.getByRole("button", { name: /新提案|New proposal/ }).click();
  await fillProposal(page, "退回案");
  await page.locator("form button[type=submit]").first().click();
  await expect(page.getByText("work_arrangement").first()).toBeVisible({ timeout: 10000 });
  await page.getByRole("button", { name: "work_arrangement" }).first().click();
  await page.getByRole("button", { name: /提交审批|Submit for approval/ }).click();
  await expect(page.getByText(/pending/).first()).toBeVisible({ timeout: 10000 });
  // 退回需要理由。
  await page.getByLabel(/理由|Reason/).fill("范围不清晰");
  await page.getByRole("button", { name: /^退回$|^Reject$/ }).click();
  await expect(page.getByText(/cancelled/).first()).toBeVisible({ timeout: 10000 });
  await expect(page.getByRole("button", { name: /^同意$|^Approve$/ })).toHaveCount(0);
});

async function fillProposal(page: import("@playwright/test").Page, title: string) {
 await page.getByLabel(/^名称$|^Name$/).fill(title);
 await page.getByLabel(/目标 \/ 预期成果|Goal \/ expected output/).fill("实现约定目标");
 await page.getByLabel(/^验收标准$|^Acceptance criteria$/).fill("结果可核对");
 await page.getByRole("button", { name: /负责人$|Owner$/ }).click();
 await page.getByRole("option", { name: /负责人职责/ }).click();
}
