// P6-03 提案会签全链路（B 系列核心路径），统一工作区（ChatWorkspace 树）UI。
// 前置：项目由各用例自建；职位/流程/身份经 API 准备（统一工作区的编辑入口在后续阶段接入）。
import { expect, test } from "@playwright/test";

const stamp = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);

async function api(page: import("@playwright/test").Page, method: "get" | "post", route: string, body?: unknown) {
  const session = (await (await page.request.get("/api/v1/auth/session")).json()).data;
  const headers = { "X-CSRF-Token": session.csrfToken, "Idempotency-Key": crypto.randomUUID() };
  const response = await (method === "get"
    ? page.request.get("/api/v1" + route)
    : page.request.post("/api/v1" + route, { headers, data: body }));
  expect(response.ok(), await response.text()).toBeTruthy();
  return (await response.json()).data;
}

// 注册→建项目→API 准备 流程（发布）+职位（绑定节点）+身份。返回项目 id。
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
  // 创建后自动进入统一工作区
  await expect(page.getByTestId("project-switcher")).toBeVisible({ timeout: 20000 });
  const projects = await api(page, "get", "/projects");
  const project = projects.items.find((p: any) => p.title === `提案项目-${name}`);
  const workflow = await api(page, "post", `/projects/${project.id}/workflows`, {
    name: "交付流程",
    instructions: "成员回报，接收人明确接收。",
    nodes: [
      { id: "make", name: "准备与上报", responsibility: "实现并回报", allowedPositionIds: [], defaultApprovalPolicy: "all" },
    ],
    advisoryEdges: [],
  });
  const draft = (await api(page, "get", `/projects/${project.id}/workflows/${workflow.id}/versions`)).items[0];
  await api(page, "post", `/projects/${project.id}/workflows/${workflow.id}/publish`, {
    expectedVersion: 1,
    draftHash: draft.draftHash,
  });
  const position = await api(page, "post", `/projects/${project.id}/positions`, {
    name: "负责人职责",
    prompt: "核对计划",
    nodeBindings: [{ workflowId: workflow.id, nodeId: "make" }],
  });
  const session = (await (await page.request.get("/api/v1/auth/session")).json()).data;
  await api(page, "post", `/projects/${project.id}/identities`, { positionId: position.id, userId: session.user.id });
  return project.id as string;
}

// 在议题里整理出工作提案并确认发送；返回提案 id。
async function proposeInTopic(page: import("@playwright/test").Page, title: string) {
  await page.getByTestId("new-discussion").click();
  await page.getByTestId("discussion-title").fill(title);
  await page.getByTestId("create-discussion").click();
  await expect(page.getByTestId("work-discussion-input")).toBeVisible({ timeout: 10000 });
  await page.getByTestId("chat-propose").click();
  await page.getByTestId("proposal-title").fill(title);
  await page.getByTestId("proposal-criteria").fill("结果可核对");
  await page.getByTestId("send-proposal").click();
  // 提案卡出现在议题会话里
  await expect(page.getByText(/等待确认|Awaiting decisions/).first()).toBeVisible({ timeout: 15000 });
  const proposals = await api(page, "get", `/projects/${await currentProjectId(page)}/proposals`);
  return proposals.items[0].id as string;
}

async function currentProjectId(page: import("@playwright/test").Page) {
  return new URL(page.url()).searchParams.get("project")!;
}

test("B03/B06 提案创建→提交→唯一审批人同意→计划生效", async ({ page }) => {
  const projectId = await registerAndCreateProject(page, "solo");
  const proposalId = await proposeInTopic(page, "正式计划-甲");
  // 等待提交后审阅冻结，再同意（覆盖全部席位）
  await expect
    .poll(async () => (await api(page, "get", `/projects/${projectId}/proposals/${proposalId}/review`)).status, { timeout: 15000 })
    .toBe("pending");
  await page.getByRole("button", { name: /同意此版本|Approve this version/ }).click();
  await expect
    .poll(async () => (await api(page, "get", `/projects/${projectId}/proposals/${proposalId}/review`)).status, { timeout: 15000 })
    .toBe("approved");
  // 计划生效（active）且出现在任务路线里
  await expect
    .poll(async () => (await api(page, "get", `/projects/${projectId}/plans`)).items.map((p: any) => p.status).join(","), { timeout: 15000 })
    .toContain("active");
  await page.getByTestId("work-nav-plans").click();
  await expect(page.getByText("正式计划-甲").first()).toBeVisible();
});

test("B16 已审批提案不可重复决定", async ({ page }) => {
  const projectId = await registerAndCreateProject(page, "dup");
  const proposalId = await proposeInTopic(page, "退回案");
  await expect
    .poll(async () => (await api(page, "get", `/projects/${projectId}/proposals/${proposalId}/review`)).status, { timeout: 15000 })
    .toBe("pending");
  // 退回需要理由
  await page.getByRole("button", { name: /退回并说明|Return with feedback/ }).first().click();
  await page.getByLabel(/需要修改的内容|What needs to change/).fill("范围不清晰");
  await page
    .locator(".judex-dialog-content")
    .getByRole("button", { name: /退回并说明|Return with feedback/ })
    .click();
  await expect
    .poll(async () => (await api(page, "get", `/projects/${projectId}/proposals/${proposalId}/review`)).status, { timeout: 15000 })
    .toBe("cancelled");
  // 卡片显示已退回，且不再出现同意按钮
  await expect(page.getByText(/已退回|Returned/).first()).toBeVisible({ timeout: 10000 });
  await expect(page.getByRole("button", { name: /同意此版本|Approve this version/ })).toHaveCount(0);
});
