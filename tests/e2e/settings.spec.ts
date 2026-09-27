import {chooseValue} from "./workspace-helpers";
import { test, expect } from "@playwright/test";
import {
  openAccount,
  openSettings,
  openTool,
  selectPerson,
} from "./workspace-helpers";
test("account menu owns appearance, invitation and sign-out; settings are a separate reversible page", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/?project=leaf&view=topic&item=labels");
  await expect(page.getByTestId("chat-new-tab")).toHaveCount(0);
  await expect(page.getByTestId("new-discussion")).toHaveText("新建讨论");
  await expect(page.getByTestId("next-theme")).toHaveCount(0);
  await page.getByTestId("work-discussion-input").fill("返回后仍保留");
  await openTool(page, "plans");
  await openAccount(page);
  await expect(page.getByTestId("account-invite")).toBeDisabled();
  await expect(page.getByTestId("account-logout")).toBeVisible();
  await page.getByTestId("next-theme").click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByTestId("account-settings").click();
  await expect(page.getByTestId("settings-page")).toBeVisible();
  await expect(page.locator(".judex-chat-app")).toBeHidden();
  await chooseValue(page,"settings-language","en");
  await expect(
    page.getByRole("heading", { name: "General", exact: true }),
  ).toBeVisible();
  await page.getByTestId("settings-section-preferences").click();
  await page.getByTestId("next-personal-prompt").fill("A draft in settings");
  await page.screenshot({
    path: test.info().outputPath("settings-dark-en.png"),
    animations: "disabled",
  });
  await page.getByTestId("settings-back").click();
  await expect(page.getByTestId("work-discussion-input")).toHaveValue(
    "返回后仍保留",
  );
  await expect(page.getByTestId("workspace-tab-plans")).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await openSettings(page, "preferences");
  await expect(page.getByTestId("next-personal-prompt")).toHaveValue(
    "A draft in settings",
  );
  await page.reload();
  await expect(page.getByTestId("settings-page")).toBeVisible();
  await expect(page.getByTestId("next-personal-prompt")).toHaveValue(
    "A draft in settings",
  );
  expect(errors).toEqual([]);
});
test("managers can invite from the account menu, project configuration is not a work tab", async ({
  page,
}) => {
  await page.goto("/");
  await selectPerson(page, "林然");
  await openAccount(page);
  await page.getByTestId("account-invite").click();
  await page.getByTestId("next-invite-name").fill("新伙伴");
  await page.getByTestId("invite-position-build-role").check();
  await page.getByTestId("next-create-invite").click();
  const s = await page.evaluate(() =>
    JSON.parse(localStorage.getItem("judex.web.preview.v1")!),
  );
  expect(s.invites.some((i: any) => i.person === "新伙伴")).toBe(true);
  await page.getByTestId("workspace-add-tool").click();
  for (const id of [
    "preferences",
    "project-settings",
    "team",
    "flows",
    "local",
  ])
    await expect(page.getByTestId("work-nav-" + id)).toHaveCount(0);
  await openSettings(page, "project");
  await expect(page.getByTestId("project-discussion-limit")).toBeEnabled();
  await page.getByTestId("settings-back").click();
  await expect(page.getByTestId("workspace-tab-settings:project")).toHaveCount(
    0,
  );
});
test("leaving preview does not erase work and cannot silently re-enter on reload", async ({
  page,
}) => {
  await page.goto("/");
  const before = await page.evaluate(() =>
    localStorage.getItem("judex.web.preview.v1"),
  );
  await openAccount(page);
  await page.getByTestId("account-logout").click();
  await expect(page.getByText("已退出交互预览", { exact: true })).toBeVisible();
  await expect(page.locator(".judex-chat-app")).toHaveCount(0);
  await page.reload();
  await expect(page.getByText("已退出交互预览", { exact: true })).toBeVisible();
  expect(
    await page.evaluate(() => localStorage.getItem("judex.web.preview.v1")),
  ).toBe(before);
  await page.getByTestId("return-preview").click();
  await expect(page.getByTestId("account-menu-button")).toBeVisible();
});
