import {openTaskAction,selectRouteTask} from './workspace-helpers';
import {test,expect,type Page} from "@playwright/test";
import {seedWork} from "../../web/src/features/work/seed";
import {ensureDemoMainTopics} from "../../web/src/features/chat/demoCollaboration";
import {chooseValue,selectPerson} from "./workspace-helpers";

async function state(page:Page){return page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!));}
async function enter(page:Page){await page.goto("/");await page.getByTestId("project-enter-leaf").click();await expect(page.getByTestId("hub-tab-plans")).toBeVisible();}
test.beforeEach(async({page},info)=>{
 const seed=ensureDemoMainTopics(seedWork());seed.currentUser="林然";if(info.title.includes("D02"))seed.tasks.push({...structuredClone(seed.tasks.find(t=>t.id==="build")!),id:"second-plan-task",planId:"leaf-next",status:"draft",title:{zh:"第二计划任务",en:"Second plan task"},requirements:[],files:[]});if(info.title.includes("D03"))seed.tasks.find(t=>t.id==="build")!.status="working";
 await page.addInitScript(s=>{if(!localStorage.getItem("judex.web.preview.v1"))localStorage.setItem("judex.web.preview.v1",JSON.stringify(s));},seed);
});
test("COOP-D01 cards separate route, preview and discussions; task main is lazy and reused",async({page})=>{
 await enter(page);const before=(await state(page)).topics.length;
 await page.getByTestId("plan-preview-leaf-first").click();await expect(page.getByTestId("route-fullscreen")).toBeVisible();await expect(page.getByTestId("execution-task-build")).toBeVisible();await page.keyboard.press("Escape");await expect(page.getByTestId("hub-tab-plans")).toBeVisible();
 await page.getByTestId("plan-card-leaf-first").locator(".judex-co-card-hit").click();await expect(page).toHaveURL(/\/plans\/leaf-first\/route/);expect((await state(page)).topics.length).toBe(before);
 await page.getByTestId("task-discuss-build").click();await expect(page).toHaveURL(/\/tasks\/build\/chat\/task-main-build/);await expect(page.getByTestId("scoped-topic-task-main-build")).toBeVisible();expect((await state(page)).topics.length).toBe(before+1);
 await page.getByTestId("work-discussion-input").fill("任务讨论草稿");await page.getByTestId("workspace-back-projects").click();await page.getByTestId("task-discuss-build").click();expect((await state(page)).topics.length).toBe(before+1);await expect(page.getByTestId("work-discussion-input")).toHaveValue("任务讨论草稿");
});
test("COOP-D02 one shared conversation appears across two plans and two tasks",async({page})=>{
 await enter(page);await page.getByTestId("plan-discuss-leaf-first").click();await page.getByTestId("new-discussion").click();await page.getByTestId("discussion-title").fill("跨计划协商");await page.getByTestId("topic-plan-leaf-next").click();await page.getByTestId("topic-task-build").click();const otherTask=(await state(page)).tasks.find((t:any)=>t.planId==="leaf-next");await page.getByTestId("topic-task-"+otherTask.id).click();await page.getByTestId("create-discussion").click();
 const shared=(await state(page)).topics.find((t:any)=>t.title.zh==="跨计划协商");expect(shared).toBeTruthy();await page.getByTestId("work-discussion-input").fill("两个计划共享这一条消息");await page.getByTestId("send-work-message").click();
 await page.goto(`/projects/leaf/plans/leaf-next/chat/${shared.id}`);await expect(page.getByTestId("scoped-topic-"+shared.id)).toBeVisible();await expect(page.getByTestId("chat-thread")).toContainText("两个计划共享这一条消息");
 await page.goto(`/projects/leaf/tasks/build/chat/${shared.id}`);await expect(page.getByTestId("scoped-topic-"+shared.id)).toBeVisible();await expect(page.getByTestId("chat-thread")).toContainText("两个计划共享这一条消息");
 const after=await state(page);expect(after.topics.filter((t:any)=>t.id===shared.id)).toHaveLength(1);expect(after.tasks.find((t:any)=>t.id==="build").planId).toBe("leaf-first");
});
test("COOP-D03 progress stays in records; a question becomes one discussion after human choice",async({page})=>{
 await enter(page);await page.getByTestId("plan-discuss-leaf-first").click();await selectPerson(page,"顾言");
 await page.locator(".judex-co-context-task").filter({hasText:"让任务创建和列表顺畅可用"}).click();
 const before=(await state(page)).topics.length;
 await openTaskAction(page,"模拟本地推送");await chooseValue(page,"collaboration-input-kind","progress");await page.getByTestId("collaboration-report-body").fill("进展补充，不增加会话");await page.getByTestId("collaboration-submit-record").click();expect((await state(page)).topics.length).toBe(before);
 await openTaskAction(page,"模拟本地推送");await chooseValue(page,"collaboration-input-kind","question");await page.getByTestId("collaboration-report-body").fill("需要明确重试策略");await page.getByTestId("collaboration-submit-record").click();expect((await state(page)).topics.length).toBe(before);
 const suggestion=(await state(page)).discussionSuggestions.at(-1);await page.getByTestId("discussion-suggestion-"+suggestion.id).getByRole("button",{name:"单独讨论",exact:true}).click();await page.getByTestId("collaboration-resolve-suggestion").click();
 await expect.poll(async()=> (await state(page)).topics.length).toBe(before+1);expect((await state(page)).taskActivities.some((a:any)=>a.text==="需要明确重试策略")).toBeTruthy();expect((await state(page)).tasks.find((t:any)=>t.id==="build").status).toBe("working");
});
test("COOP-D04 complete settings and private role drafts remain available through project controls",async({page})=>{
 await enter(page);await page.getByTestId("project-settings").click();await expect(page.getByTestId("settings-page")).toBeVisible();await expect(page.getByTestId("project-discussion-limit")).toBeVisible();await expect(page.getByTestId("settings-section-preferences")).toHaveCount(0);
 await page.getByTestId("settings-section-team").click();await expect(page.getByTestId("open-position-presets")).toBeVisible();await expect(page.locator('[data-testid^="assign-position-"]').first()).toBeVisible();await page.locator('[data-testid^="position-preferences-"]').first().click();await page.getByTestId("next-personal-prompt").fill("只属于当前职位的草稿");await page.keyboard.press("Escape");await page.locator('[data-testid^="position-preferences-"]').first().click();await expect(page.getByTestId("next-personal-prompt")).toHaveValue("只属于当前职位的草稿");await page.keyboard.press("Escape");
 await page.getByTestId("settings-section-flows").click();await expect(page.getByTestId("open-workflow-presets")).toBeVisible();await page.getByTestId("settings-section-local").click();await expect(page.getByTestId("download-cli-windows")).toBeVisible();await page.getByTestId("settings-back").click();await expect(page.getByTestId("hub-tab-plans")).toBeVisible();
});
test("COOP-D05 top account appearance, full tabs, maximization and draft survive navigation",async({page})=>{
 await enter(page);await page.getByTestId("plan-discuss-leaf-first").click();await page.getByTestId("work-discussion-input").fill("持续保存的计划讨论草稿");await expect(page.locator(".judex-chat-sidebar [data-testid=account-menu-button]")).toHaveCount(0);await expect(page.getByTestId("tab-topic:main:leaf-first")).toBeVisible();
 await page.getByTestId("account-menu-button").click();await page.getByTestId("next-theme").click();await page.getByTestId("next-language").click();await page.keyboard.press("Escape");await page.setViewportSize({width:1120,height:900});await expect(page.locator("html")).toHaveAttribute("data-theme","dark");
 await page.getByTestId("workspace-maximize").click();await page.getByTestId("workspace-maximize").click();await expect(page.getByTestId("work-discussion-input")).toHaveValue("持续保存的计划讨论草稿");await page.reload();await expect(page.getByTestId("work-discussion-input")).toHaveValue("持续保存的计划讨论草稿");
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
test("COOP-D06 deliveries and decisions distinguish receipt from task and plan acceptance",async({page})=>{
 await enter(page);await page.getByTestId("hub-tab-deliveries").click();await expect(page.getByTestId("delivery-card-first-review")).toBeVisible();await page.getByTestId("hub-tab-decisions").click();await expect(page.getByTestId("decision-open-first-review")).toHaveCount(0);
 await selectPerson(page,"周宁");await page.getByTestId("hub-tab-decisions").click();await page.getByTestId("decision-open-first-review").click();await page.getByTestId("receive-source-api").click();await expect.poll(async()=> (await state(page)).handoffs.find((h:any)=>h.id==="first-review").sources.find((s:any)=>s.id==="source-api").status).toBe("accepted");expect((await state(page)).tasks.find((t:any)=>t.id==="build").status).toBe("delivered");
});
test("COOP-D07 filtering and panel resizing keep the active topic, draft and reading position",async({page})=>{
 await enter(page);await page.getByTestId("plan-discuss-leaf-first").click();await page.getByTestId("work-discussion-input").fill("筛选不会切走当前讨论");
 const before=page.url();await page.locator(".judex-collab-search input").fill("无匹配的讨论");await expect(page.locator("[data-testid^=scoped-topic-]")).toHaveCount(0);expect(page.url()).toBe(before);await expect(page.getByTestId("work-discussion-input")).toHaveValue("筛选不会切走当前讨论");
 await page.getByTestId("resize-left").focus();await page.keyboard.press("ArrowRight");await page.keyboard.press("ArrowRight");await expect(page.getByTestId("work-discussion-input")).toHaveValue("筛选不会切走当前讨论");
 await page.getByTestId("chat-thread").evaluate(el=>el.scrollTop=0);await page.reload();await expect(page.getByTestId("work-discussion-input")).toHaveValue("筛选不会切走当前讨论");expect(await page.getByTestId("chat-thread").evaluate(el=>el.scrollTop)).toBe(0);
});
test("COOP-D08 fixed message fork preserves source identity and independent suffixes",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/chat/labels");const before=await state(page),original=before.topics.find((t:any)=>t.id==="labels");
 const first=page.locator('[data-message-seq="1"]');await expect(first).toBeVisible();await first.getByRole("button",{name:/分叉/}).click();await page.getByRole("menuitem",{name:"从此处分叉",exact:true}).click();await page.getByRole("dialog").getByRole("button",{name:/建立分支|创建分支/}).click();
 await expect(page.locator(".judex-collab-inherited").first()).toBeVisible();const branch=(await state(page)).topics.find((t:any)=>t.parentTopicId==="labels");expect(branch.forkAfterSeq).toBe(1);expect(branch.messages).toHaveLength(0);expect(original.messages).toHaveLength(2);
 await page.getByTestId("work-discussion-input").fill("分支新增内容");await page.getByTestId("send-work-message").click();await expect(page.getByTestId("chat-thread")).toContainText("分支新增内容");await page.goto("/projects/leaf/plans/leaf-first/chat/labels");await expect(page.getByTestId("chat-thread")).not.toContainText("分支新增内容");
});
