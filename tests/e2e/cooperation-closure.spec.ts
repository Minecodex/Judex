import {openTaskAction,selectRouteTask} from './workspace-helpers';
import {test,expect,type Page} from "@playwright/test";
import {seedWork} from "../../web/src/features/work/seed";
import {ensureDemoMainTopics} from "../../web/src/features/chat/demoCollaboration";
import {chooseValue,selectPerson} from "./workspace-helpers";
test.beforeEach(async({page},info)=>{
 const seed=ensureDemoMainTopics(seedWork());seed.currentUser="林然";
 if(info.title.includes("context")){seed.tasks.push({...structuredClone(seed.tasks.find(v=>v.id==="build")!),id:"cross-task",planId:"leaf-next",title:{zh:"跨计划协作任务",en:"Cross plan task"}});const topic=seed.topics.find(v=>v.id===seed.plans.find(p=>p.id==="leaf-first")!.mainTopicId)!;topic.planIds.push("leaf-next");topic.taskIds.push("cross-task");}
 if(info.title.includes("report")){seed.currentUser="顾言";seed.tasks.find(v=>v.id==="build")!.status="working";}
 await page.addInitScript(s=>{if(!localStorage.getItem("judex.web.preview.v1"))localStorage.setItem("judex.web.preview.v1",JSON.stringify(s));},seed);
});
test("COOP-D09 delivery and receipt deep links restore details and switch the actual task",async({page})=>{
 await page.goto("/projects/leaf?tab=deliveries&view=task&item=build");await expect(page.getByTestId("task-details-drawer")).toContainText("让任务创建和列表顺畅可用");await page.reload();await expect(page.getByTestId("task-details-drawer")).toContainText("让任务创建和列表顺畅可用");
 await selectRouteTask(page,"guide");await expect(page).toHaveURL(/item=guide/);await expect(page.getByTestId("task-details-drawer").locator("h2").first()).toHaveText("写好第一次使用的说明");
 await page.goto("/projects/leaf?tab=deliveries&view=handoff&item=first-review");await page.reload();await expect(page.getByRole("dialog")).toContainText("首版的两份交付");await expect(page.getByRole("dialog")).not.toContainText("任务不存在");
 await page.keyboard.press("Escape");await selectPerson(page,"周宁");await page.goto("/projects/leaf?tab=decisions&view=handoff&item=first-review");await page.reload();await expect(page.getByRole("dialog")).toContainText("首版的两份交付");
});
test("COOP-D10 context spans explicit links and removed context must be reviewed",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first");await chooseValue(page,"collaboration-compose-task","cross-task");await page.getByTestId("work-discussion-input").fill("跨对象协商草稿");await page.reload();await expect(page.getByTestId("collaboration-compose-task")).toHaveAttribute("data-value","cross-task");
 await page.getByRole("button",{name:"关联对象",exact:true}).click();await page.getByTestId("topic-task-cross-task").click();await page.getByTestId("topic-plan-leaf-next").click();await page.getByTestId("save-topic-links").click();
 await expect(page.getByText("所选任务已不在会话范围内，请重新选择。",{exact:true})).toBeVisible();await expect(page.getByTestId("send-work-message")).toBeDisabled();await expect(page.getByTestId("work-discussion-input")).toHaveValue("跨对象协商草稿");await chooseValue(page,"collaboration-compose-task","");await expect(page.getByTestId("send-work-message")).toBeEnabled();
});
test("COOP-D11 report drafts restore without leaking to another member",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/route?task=build");await openTaskAction(page,"上报进展");await page.getByTestId("collaboration-report-body").fill("正式进展的未提交草稿");await page.reload();await openTaskAction(page,"上报进展");await expect(page.getByTestId("collaboration-report-body")).toHaveValue("正式进展的未提交草稿");await page.keyboard.press("Escape");await page.keyboard.press("Escape");
 await selectPerson(page,"林然");await page.getByTestId("execution-task-build").locator(".judex-co-card-hit").click();await openTaskAction(page,"提出问题");await expect(page.getByTestId("collaboration-report-body")).toHaveValue("");
});
test("COOP-D12 graph zoom and nonzero chat reading positions restore",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/route");await page.getByRole("button",{name:"缩小",exact:true}).click();await page.getByRole("button",{name:"缩小",exact:true}).click();await page.reload();await expect(page.locator(".judex-execution-toolbar small")).toHaveText("80%");
 await page.goto("/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first");const thread=page.getByTestId("chat-thread");await expect(thread).toBeVisible();const top=await thread.evaluate(el=>{el.scrollTop=120;el.dispatchEvent(new Event("scroll"));return el.scrollTop;});expect(top).toBeGreaterThan(0);await page.reload();await expect.poll(()=>thread.evaluate(el=>el.scrollTop)).toBeCloseTo(top,0);
});
test("COOP-D13 scoped chat migrates old tools and exposes task actions without the old work page",async({page})=>{
 await page.addInitScript(()=>sessionStorage.setItem("judex.workspace.tabs.v1.leaf:林然",JSON.stringify({tabs:[{view:"overview"},{view:"handoffs"},{view:"decisions"}],active:"overview"})));
 await page.goto("/projects/leaf/plans/leaf-first/chat/main%3Aleaf-first");await expect(page.getByRole("heading",{name:"待办概览",exact:true})).toHaveCount(0);await expect(page.getByTestId("workspace-tab-decisions")).toHaveCount(0);await expect(page.getByTestId("workspace-tab-handoffs")).toHaveCount(0);await page.locator(".judex-co-context-task").filter({hasText:"写好第一次使用的说明"}).click();await page.getByTestId("workspace-add-tool").click();await expect(page.getByRole("menuitem",{name:"任务记录",exact:true})).toBeVisible();await expect(page.getByRole("menuitem",{name:"流程参考",exact:true})).toBeVisible();await page.keyboard.press("Escape");await expect(page.getByTestId("accept-task")).toBeVisible();await expect(page.locator(".judex-chat-inspector .judex-task-detail")).toHaveCount(0);
});
test("COOP-D14 a task main report exposes human suggestions and continues in the correct plan scope",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/route");await page.getByTestId("task-discuss-build").click();const before=(await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!))).topics.length;
 await openTaskAction(page,"提出问题");await page.getByTestId("collaboration-report-body").fill("登录失败，需要共同核对");await page.getByTestId("collaboration-submit-record").click();await expect(page.locator('[data-testid^="discussion-suggestion-"]')).toBeVisible();await page.getByRole("button",{name:"在主讨论继续",exact:true}).click();await expect(page).toHaveURL(/\/plans\/leaf-first\/chat\/main/);const after=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!));expect(after.topics.length).toBe(before);expect(after.tasks.find((v:any)=>v.id==="build").status).toBe("working");
});
