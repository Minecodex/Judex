import {chooseValue} from "./workspace-helpers";
import { test, expect, type Page } from "@playwright/test";
import { openTool } from "./workspace-helpers";
const bounds = async (page: Page, selector: string) =>
  (await page.locator(selector).boundingBox())!;
test("tab bars start at the top and only one new conversation entry remains", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await expect(page.locator(".judex-preview-banner")).toHaveCount(0);
  await expect(page.locator(".judex-chat-topline")).toHaveCount(0);
  await expect(page.locator(".judex-chat-panel-top")).toHaveCount(0);
  const center = await bounds(
      page,
      ".judex-chat-main .judex-conversation-tabstrip",
    ),
    right = await bounds(page, ".judex-workspace-tabstrip");
  expect(center.y).toBeCloseTo(0, 0);
  expect(right.y).toBeCloseTo(0, 0);
  await expect(page.getByTestId("chat-new-tab")).toHaveCount(0);
  await expect(page.getByTestId("chat-pane-toggle")).toBeVisible();
  expect(
    await page.evaluate(() => document.documentElement.scrollHeight),
  ).toBeLessThanOrEqual(1000);
  await page.screenshot({
    path: test.info().outputPath("top-aligned-tabs.png"),
    animations: "disabled",
  });
});
test("maximizing takes over the conversation area and restores original sizes and drafts", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await page.getByTestId("work-discussion-input").fill("中间的草稿保持");
  await openTool(page, "plans");
  await chooseValue(page,"work-map-plan-filter","leaf-first");
  const center = await bounds(page, ".judex-chat-main"),
    right = await bounds(page, ".judex-chat-inspector"),
    left = await bounds(page, ".judex-chat-sidebar");
  const preferences = await page.evaluate(() =>
    localStorage.getItem("judex.chat.layout.v1"),
  );
  await page.getByTestId("workspace-maximize").click();
  await expect(page.locator(".judex-chat-main")).toBeHidden();
  await expect(page.getByTestId("resize-right")).toHaveCount(0);
  await expect(page.getByTestId("workspace-maximize")).toHaveAttribute(
    "aria-label",
    "恢复三栏布局",
  );
  const expanded = await bounds(page, ".judex-chat-inspector");
  expect(expanded.x).toBeCloseTo(center.x, 0);
  expect(expanded.width).toBeCloseTo(center.width + right.width + 8, 0);
  expect((await bounds(page, ".judex-chat-sidebar")).width).toBeCloseTo(
    left.width,
    0,
  );
  await expect(page.getByTestId("work-map-plan-filter")).toHaveAttribute("data-value",
    "leaf-first",
  );
  await page.screenshot({
    path: test.info().outputPath("maximized-workspace.png"),
    animations: "disabled",
  });
  await page.getByTestId("workspace-maximize").click();
  await expect(page.locator(".judex-chat-main")).toBeVisible();
  expect((await bounds(page, ".judex-chat-main")).width).toBeCloseTo(
    center.width,
    0,
  );
  expect((await bounds(page, ".judex-chat-inspector")).width).toBeCloseTo(
    right.width,
    0,
  );
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "中间的草稿保持",
  );
  expect(
    await page.evaluate(() => localStorage.getItem("judex.chat.layout.v1")),
  ).toBe(preferences);
  await page.getByTestId("workspace-maximize").click();
  await page.getByRole("button", { name: "收起工作区", exact: true }).click();
  await expect(page.locator(".judex-chat-main")).toBeVisible();
  await page.getByTestId("chat-pane-toggle").click();
  await expect(page.locator(".judex-chat-main")).toBeVisible();
  expect((await bounds(page, ".judex-chat-inspector")).width).toBeCloseTo(
    right.width,
    0,
  );
});
