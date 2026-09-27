import {openTool,openSettings,selectPerson,toggleTheme,toggleLanguage} from './workspace-helpers';
import { test, expect, type Page } from "@playwright/test";
const snapshot = (page: Page) =>
  page.evaluate(() =>
    JSON.parse(localStorage.getItem("judex.web.preview.v1")!),
  );
const sidebar = (page: Page) => page.locator(".judex-chat-conversation-list");
test("conversations open reusable tabs and retain separate drafts, attachments and reading positions", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/?project=leaf&view=topic&item=labels");
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(2);
  await page.getByTestId("work-discussion-input").fill("首个聊天草稿");
  await page.getByRole("button", { name: "添加资料", exact: true }).click();
  await page.getByTestId("work-files").setInputFiles({
    name: "未发送.md",
    mimeType: "text/markdown",
    buffer: Buffer.from("留在此聊天"),
  });
  await page.getByTestId("chat-discuss").click();
  await page.getByTestId("chat-thread").evaluate((el) => {
    el.scrollTop = 180;
    el.dispatchEvent(new Event("scroll"));
  });
  await page.getByTestId("new-discussion").click();
  await page.getByTestId("discussion-title").fill("第二个聊天");
  await page.getByTestId("create-discussion").click();
  await page.getByTestId("work-discussion-input").fill("第二份草稿");
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(3);
  await sidebar(page)
    .getByRole("button", { name: "首版需要标签吗？", exact: true })
    .click();
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "首个聊天草稿",
  );
  await expect(page.getByText("未发送.md", { exact: true })).toBeVisible();
  expect(
    await page.getByTestId("chat-thread").evaluate((el) => el.scrollTop),
  ).toBeCloseTo(180, 0);
  await sidebar(page)
    .getByRole("button", { name: "首版需要标签吗？", exact: true })
    .click();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(3);
  await page.locator(".judex-chat-main").getByRole("tab", { name: "第二个聊天", exact: true }).click();
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "第二份草稿",
  );
  await page.reload();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(3);
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "第二份草稿",
  );
  expect(errors).toEqual([]);
});
test("closing a tab never locks discussion and work tools preserve a handoff tab", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await expect(page.getByTestId("close-pure-topic")).toHaveCount(0);
  await expect(page.getByTestId("work-discussion-input")).toBeEnabled();
  const before = await snapshot(page);
  await sidebar(page)
    .getByRole("button", { name: /首版的两份交付/ })
    .click();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(3);
  await openTool(page,"plans");
  await expect(page.getByTestId("contribution-source-api")).toBeVisible();
  await page
    .getByRole("button", { name: "关闭标签 · 首版需要标签吗？", exact: true })
    .click();
  await expect(page.getByTestId("tab-handoff:first-review")).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(2);
  await page
    .getByRole("button", {
      name: "关闭标签 · 首版的两份交付，等你看一眼",
      exact: true,
    })
    .click();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(1);
  const after = await snapshot(page);
  expect(after.topics).toEqual(before.topics);
  expect(after.tasks).toEqual(before.tasks);
  expect(after.handoffs).toEqual(before.handoffs);
  await sidebar(page)
    .getByRole("button", { name: "首版需要标签吗？", exact: true })
    .click();
  await expect(page.getByTestId("work-discussion-input")).toBeEnabled();
  await page.getByTestId("tab-topic:labels").focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.getByTestId("tab-home")).toHaveAttribute(
    "aria-selected",
    "true",
  );
});
test("project tabs are isolated and the tools menu has understandable destinations", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await page.getByTestId("project-switcher").click();
  await page.getByRole("option", { name: "野间咖啡", exact: true }).click();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(1);
  await sidebar(page)
    .getByRole("button", { name: "怎样表达自然的松弛感？", exact: true })
    .click();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(2);
  await page.getByTestId("project-switcher").click();
  await page.getByRole("option", { name: "轻笺", exact: true }).click();
  await expect(page.locator(".judex-chat-main").getByRole("tab")).toHaveCount(2);
  await expect(
    page.locator(".judex-chat-main").getByRole("tab", { name: "怎样表达自然的松弛感？" }),
  ).toHaveCount(0);
  await openTool(page,"local");
  await expect(page.locator(".judex-settings-content")).toContainText(
    "尚未接入真实",
  );
  await page.screenshot({
    path: test.info().outputPath("browser-tabs.png"),
    animations: "disabled",
  });
});
