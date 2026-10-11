import {openTool,openSettings,selectPerson,toggleTheme,toggleLanguage} from './workspace-helpers';
import { test, expect, type Page } from "@playwright/test";
const width = async (page: Page, selector: string) =>
  (await page.locator(selector).boundingBox())!.width;
const sizes = async (page: Page) => ({
  left: await width(page, ".judex-chat-sidebar"),
  center: await width(page, ".judex-chat-main"),
  right: await width(page, ".judex-chat-inspector"),
});
async function drag(page: Page, side: string, delta: number) {
  const box = (await page.getByTestId("resize-" + side).boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + 100);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2 + delta, box.y + 100, {
    steps: 8,
  });
  await page.mouse.up();
}
test("both dividers resize independently, save ratios, restore on refresh and never remount the conversation", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await page.getByTestId("work-discussion-input").fill("拖动时保留我的草稿");
  const initial = await sizes(page);
  await drag(page, "left", 64);
  let current = await sizes(page);
  expect(current.left).toBeCloseTo(initial.left + 64, 0);
  expect(current.right).toBeCloseTo(initial.right, 0);
  await drag(page, "right", -72);
  current = await sizes(page);
  expect(current.right).toBeCloseTo(initial.right + 72, 0);
  expect(current.center).toBeGreaterThanOrEqual(359);
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "拖动时保留我的草稿",
  );
  const saved = await sizes(page);
  await page.reload();
  await expect(page.getByTestId("chat-thread")).toBeVisible();
  expect((await sizes(page)).left).toBeCloseTo(saved.left, 0);
  expect((await sizes(page)).right).toBeCloseTo(saved.right, 0);
  await page.setViewportSize({ width: 1120, height: 900 });
  await expect.poll(async () => (await sizes(page)).center).toBeGreaterThanOrEqual(359);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect
    .poll(() => width(page, ".judex-chat-sidebar"))
    .toBeCloseTo(saved.left, 0);
  await page.getByRole("button", { name: "专注聊天", exact: true }).click();
  await expect(page.getByTestId("resize-right")).toHaveCount(0);
  await page.getByRole("button", { name: "展开工作区", exact: true }).click();
  expect((await sizes(page)).right).toBeCloseTo(saved.right, 0);
});
test("keyboard resizing, cancellation and reset keep the layout usable", async ({
  page,
}) => {
  await page.goto("/projects/leaf/plans/leaf-first/chat/labels");
  const initial = await sizes(page);
  const left = page.getByTestId("resize-left");
  await left.focus();
  await page.keyboard.press("ArrowRight");
  expect((await sizes(page)).left).toBeCloseTo(initial.left + 12, 0);
  const before = await sizes(page),
    box = (await left.boundingBox())!;
  await page.mouse.move(box.x + 4, box.y + 80);
  await page.mouse.down();
  await page.mouse.move(box.x + 90, box.y + 80);
  await page.keyboard.press("Escape");
  await page.mouse.up();
  expect((await sizes(page)).left).toBeCloseTo(before.left, 0);
  await left.dblclick();
  expect((await sizes(page)).left).toBeCloseTo(initial.left, 0);
  await page.getByTestId("resize-right").focus();
  await page.keyboard.press("Home");
  expect((await sizes(page)).right).toBeCloseTo(260, 0);
  await openTool(page,"resources");
  await expect(page.locator(".judex-chat-panel-body")).toBeVisible();
  await toggleTheme(page);
  await toggleLanguage(page);
  await page.screenshot({
    path: test.info().outputPath("resized-preferences-dark.png"),
    fullPage: true,
    animations: "disabled",
  });
});
test("project dropdown replaces the brand and selection spans the whole conversation row", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.locator(".judex-chat-brand")).toHaveCount(0);
  await expect(page.locator(".judex-chat-projects")).toHaveCount(0);
  await page.getByTestId("project-enter-wild").click();await page.getByRole("button",{name:"项目讨论记录",exact:true}).click();await expect(page.locator(".judex-chat-conversation-list")).toContainText("怎样表达自然的松弛感");await page.locator(".judex-co-brand").click();await page.getByTestId("project-enter-leaf").click();await page.getByTestId("plan-discuss-leaf-first").click();
  const list = page.locator(".judex-chat-conversation-list");
  const buttons = list.getByRole("button");
  for (let i = 0; i < (await buttons.count()); i++) {
    await buttons.nth(i).click();
    const selected = list.locator(".judex-chat-selected");
    expect((await selected.boundingBox())!.width).toBeCloseTo(
      (await list.boundingBox())!.width,
      0,
    );
  }
  expect(await list.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(
    true,
  );
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.screenshot({
    path: test.info().outputPath("sidebar-reference.png"),
    fullPage: true,
    animations: "disabled",
  });
});
test("project dialog is spacious with a top-right close control", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "新建项目", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await page.getByTestId("project-project-name").fill("新的项目");
  await expect
    .poll(async () => (await dialog.boundingBox())!.width)
    .toBeGreaterThanOrEqual(740);
  const box = (await dialog.boundingBox())!;
  expect(box.width).toBeGreaterThanOrEqual(740);
  const close = dialog.getByRole("button", { name: "关闭", exact: true }),
    button = (await close.boundingBox())!;
  expect(button.x).toBeGreaterThan(box.x + box.width - 100);
  expect(button.y).toBeLessThan(box.y + 70);
  await page.getByTestId("project-project-name").fill("新的项目");
  await page.screenshot({
    path: test.info().outputPath("new-project-dialog.png"),
    fullPage: true,
    animations: "disabled",
  });
  await close.click();
  await expect(dialog).toHaveCount(0);
});
