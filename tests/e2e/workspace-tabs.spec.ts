import {chooseValue} from "./workspace-helpers";
import { test, expect } from "@playwright/test";
import {
  openTool,
  openSettings,
  selectPerson,
  toggleTheme,
  toggleLanguage,
} from "./workspace-helpers";
test("workspace tools preserve their filters, reading position and central chat", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await expect(page.getByTestId("workspace-launcher")).toBeVisible();
  await page.getByTestId("work-discussion-input").fill("中央草稿不受影响");
  await openTool(page, "plans");
  await chooseValue(page,"work-map-plan-filter","leaf-first");
  await openTool(page, "resources");
  await page.getByTestId("workspace-tab-plans").click();
  await expect(page.getByTestId("work-map-plan-filter")).toHaveAttribute("data-value",
    "leaf-first",
  );
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "中央草稿不受影响",
  );
  await page.setViewportSize({ width: 1440, height: 700 });
  const reading = await page
    .locator(".judex-chat-panel-body")
    .evaluate((el) => {
      el.scrollTop = 100;
      el.dispatchEvent(new Event("scroll"));
      return el.scrollTop;
    });
  expect(reading).toBeGreaterThan(0);
  await page.getByTestId("workspace-tab-resources").click();
  await page.getByTestId("workspace-tab-plans").click();
  await expect
    .poll(() =>
      page.locator(".judex-chat-panel-body").evaluate((el) => el.scrollTop),
    )
    .toBeCloseTo(reading, 0);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await openTool(page, "resources");
  await expect(page.getByTestId("workspace-tab-resources")).toHaveCount(1);
  await page.reload();
  await page.getByTestId("workspace-tab-plans").click();
  await expect(page.getByTestId("work-map-plan-filter")).toHaveAttribute("data-value",
    "leaf-first",
  );
  await page
    .getByRole("button", { name: "关闭功能标签 · 共享资料", exact: true })
    .click();
  await expect(page.getByTestId("workspace-tab-plans")).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page
    .getByRole("button", { name: "关闭功能标签 · 任务路线", exact: true })
    .click();
  await expect(page.getByTestId("workspace-launcher")).toBeVisible();
});
test("details use separate compact tabs with backgrounds and project switching isolates workspace tabs", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await openTool(page, "plans");
  await page.getByTestId("execution-task-build").click();
  await expect(page.getByTestId("workspace-tab-task:build")).toBeVisible();
  await page.getByTestId("workspace-tab-plans").click();
  await expect(page.getByTestId("execution-map")).toBeVisible();
  const tabs = page.getByRole("tab");
  for (let i = 0; i < (await tabs.count()); i++) {
    const tab = tabs.nth(i),
      box = (await tab.boundingBox())!;
    expect(box.width).toBeLessThanOrEqual(181);
    expect(box.height).toBeLessThanOrEqual(36);
    const color = await tab.evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    );
    expect(color).not.toBe("rgba(0, 0, 0, 0)");
  }
  await page.screenshot({
    path: test.info().outputPath("compact-workspace-tabs.png"),
    animations: "disabled",
  });
  await page.getByTestId("project-switcher").click();
  await page.getByRole("option", { name: "野间咖啡", exact: true }).click();
  await expect(page.getByTestId("workspace-tab-task:build")).toHaveCount(0);
  await expect(page.getByTestId("workspace-launcher")).toBeVisible();
  await toggleTheme(page);
  await toggleLanguage(page);
  await page.setViewportSize({ width: 1120, height: 900 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: test.info().outputPath("tool-launcher-dark.png"),
    animations: "disabled",
  });
});
