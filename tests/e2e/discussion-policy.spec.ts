import {openTool,openSettings,selectPerson,toggleTheme,toggleLanguage} from './workspace-helpers';
import { test, expect } from "@playwright/test";
test("project round limit persists, applies per submission, and never disables chat", async ({
  page,
}) => {
  await page.goto("/?project=leaf&view=topic&item=labels");
  await selectPerson(page,"林然");
  await openTool(page,"project-settings");
  const field = page.getByTestId("project-discussion-limit");
  await field.fill("2");
  await page.getByTestId("save-discussion-limit").click();
  await page.reload();
  await expect(field).toHaveValue("2");
  await field.fill("0");
  await expect(page.getByTestId("save-discussion-limit")).toBeDisabled();
  await field.fill("2");
  await page.getByTestId("settings-back").click();
  await page
    .locator(".judex-chat-conversation-list")
    .getByRole("button", { name: /首版需要标签吗/ })
    .click();
  await expect(page.getByTestId("close-pure-topic")).toHaveCount(0);
  await page.getByTestId("work-discussion-input").fill("请核对这个新提交");
  await page.getByTestId("send-work-message").click();
  await expect(page.getByTestId("discussion-budget")).toContainText("1 / 2");
  await page.getByTestId("chat-discuss").click();
  await expect(page.getByTestId("discussion-budget")).toContainText("2 / 2");
  await expect(page.getByTestId("chat-discuss")).toBeDisabled();
  await expect(page.getByTestId("work-discussion-input")).toBeEnabled();
  await page.getByTestId("conversation-menu").click();await expect(page.getByTestId("chat-propose")).toBeEnabled();await page.keyboard.press("Escape");
  await openSettings(page,"project");
  await field.fill("4");
  await page.getByTestId("save-discussion-limit").click();
  await page.getByTestId("settings-back").click();
  await expect(page.getByTestId("discussion-budget")).toContainText("2 / 2");
  await page.getByTestId("work-discussion-input").fill("这是新增的依据");
  await page.getByTestId("send-work-message").click();
  await expect(page.getByTestId("discussion-budget")).toContainText("1 / 4");
  await expect(page.getByTestId("chat-discuss")).toBeEnabled();
  await page.screenshot({
    path: test.info().outputPath("discussion-limit.png"),
    animations: "disabled",
  });
  await selectPerson(page,"顾言");
  await page.goto("/?project=leaf&view=settings&item=project");
  await expect(page.getByTestId("project-discussion-limit")).toBeDisabled();
  await expect(page.getByTestId("save-discussion-limit")).toHaveCount(0);
});
