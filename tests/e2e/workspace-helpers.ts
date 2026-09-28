import type { Page } from "@playwright/test";
export async function openTool(page: Page, id: string) {
  const settings: Record<string, string> = {
    preferences: "preferences",
    "project-settings": "project",
    team: "team",
    flows: "flows",
    local: "local",
  };
  if (settings[id]) {
    await openSettings(page, settings[id]);
    return;
  }
  if (await page.getByTestId("settings-page").isVisible())
    await page.getByTestId("settings-back").click();
  await page.getByTestId("workspace-add-tool").click();
  await page.getByTestId("work-nav-" + id).click();
}
export async function openAccount(page: Page) {
  // team/flows 等视图会重定向到全屏设置页；设置页检测必须等渲染稳定，
  // 否则懒加载竞态下跳过返回按钮，账户按钮始终不可见。
  const onSettings = await page
    .getByTestId("settings-page")
    .waitFor({ state: "visible", timeout: 4000 })
    .then(
      () => true,
      () => false,
    );
  if (onSettings) await page.getByTestId("settings-back").click();
  if (
    (await page
      .getByTestId("account-menu-button")
      .getAttribute("aria-expanded")) !== "true"
  )
    await page.getByTestId("account-menu-button").click();
}
export async function openSettings(page: Page, section = "general") {
  if (!(await page.getByTestId("settings-page").isVisible())) {
    await openAccount(page);
    await page.getByTestId("account-settings").click();
  }
  await page.getByTestId("settings-section-" + section).click();
}
export async function selectPerson(page: Page, name: string) {
  await openAccount(page);
  if (!(await page.getByTestId("work-person").isVisible()))
    await page
      .locator(".judex-account-preview")
      .getByRole("button", { name: "体验身份", exact: true })
      .click();
  await chooseValue(page, "work-person", name);
  if (await page.getByTestId("account-settings").isVisible()) await page.keyboard.press("Escape");
  await page.getByTestId("account-settings").waitFor({ state: "hidden" });
}
export async function toggleTheme(page: Page) {
  await openAccount(page);
  await page.getByTestId("next-theme").click();
  await page.keyboard.press("Escape");
  await page.getByTestId("account-settings").waitFor({ state: "hidden" });
}
export async function toggleLanguage(page: Page) {
  await openAccount(page);
  await page.getByTestId("next-language").click();
  await page.keyboard.press("Escape");
  await page.getByTestId("account-settings").waitFor({ state: "hidden" });
}
export async function chooseValue(page: Page, testId: string, value: string) {
  await page.getByTestId(testId).click();
  await page
    .locator("[data-option-value=" + JSON.stringify(value) + "]")
    .click();
  await page.getByRole("listbox").waitFor({ state: "hidden" });
}
