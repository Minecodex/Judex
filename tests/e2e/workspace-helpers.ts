import type { Page } from "@playwright/test";
export async function selectRouteTask(page:Page,id:string){const url=new URL(page.url());if(url.pathname.endsWith('/route')){const hit=page.getByTestId('execution-task-'+id).locator('.judex-co-card-hit');await hit.focus();await hit.press('Enter');}else{url.searchParams.set('item',id);await page.goto(url.toString());}await page.getByTestId('task-inspector').getByTestId('task-more-'+id).waitFor({state:'visible'});}
export async function openTaskAction(page:Page,label:string){
 const surface=page.getByTestId('task-inspector'),button=surface.getByRole('button',{name:label,exact:true});
 await surface.waitFor({state:'visible'});const url=new URL(page.url()),id=url.searchParams.get('task')??(url.searchParams.get('view')==='task'?url.searchParams.get('item'):null);if(id)await surface.getByTestId('task-more-'+id).waitFor({state:'visible'});
 if(await button.isVisible()){await button.click();return;}
 await surface.locator('[data-testid^="task-more-"]').click();await page.getByRole('menuitem',{name:label,exact:true}).click();
}
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
  await page.getByRole("menu").getByRole("menuitem",{name:id==="plans"?"计划信息":id==="resources"?"共享资料":id,exact:true}).click();
}
export async function openAccount(page: Page) {
  if (
    (await page
      .getByTestId("account-menu-button")
      .getAttribute("aria-expanded")) !== "true"
  )
    await page.getByTestId("account-menu-button").click();
}
export async function openSettings(page:Page,section="project"){
 if(!await page.getByTestId("settings-page").isVisible()){
  if(await page.getByTestId("conversation-menu").isVisible()){await page.getByTestId("conversation-menu").click();await page.getByTestId("conversation-project-settings").click();}
  else{
  if(!await page.getByTestId("project-settings").isVisible())await page.locator(".judex-co-crumb").first().click();
  await page.getByTestId("project-settings").click();
  }
 }
 const target=section==="general"?"project":section==="preferences"?"team":section;
 await page.getByTestId("settings-section-"+target).click();
 if(section==="preferences")await page.locator('[data-testid^="position-preferences-"]').first().click();
}
export async function selectPerson(page: Page, name: string) {
  await openAccount(page);
  if (!(await page.getByTestId("work-person").isVisible()))
    await page
      .locator(".judex-account-preview")
      .getByRole("button", { name: "体验身份", exact: true })
      .click();
  await chooseValue(page, "work-person", name);
  await page.keyboard.press("Escape");
}
export async function toggleTheme(page: Page) {
  await openAccount(page);
  await page.getByTestId("next-theme").click();
  await page.keyboard.press("Escape");

}
export async function toggleLanguage(page: Page) {
  await openAccount(page);
  await page.getByTestId("next-language").click();
  await page.keyboard.press("Escape");

}
export async function chooseValue(page: Page, testId: string, value: string) {
  await page.getByTestId(testId).click();
  await page
    .locator("[data-option-value=" + JSON.stringify(value) + "]")
    .click();
  await page.getByRole("listbox").waitFor({ state: "hidden" });
}
