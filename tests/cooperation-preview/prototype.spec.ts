import {test,expect,type Page} from "@playwright/test";
const key="judex.cooperation-preview.v1";
const state=(page:Page)=>page.evaluate(k=>JSON.parse(localStorage.getItem(k)!),key);
async function choose(page:Page,testId:string,value:string){await page.getByTestId(testId).click();await page.locator('[data-option-value="'+value+'"]').click();}
async function person(page:Page,name:string){await page.getByTestId("co-account").click();await page.getByRole("menuitem",{name:"体验身份 · "+name,exact:true}).click();}
test("project cards, plan route and task discussion have separate entrances; task discussion is created once",async({page},info)=>{
 const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
 await page.goto("/demo.html");await expect(page.getByTestId("co-project-launch")).toBeVisible();
 await page.getByTestId("co-project-launch").click();await expect(page.getByTestId("co-tab-plans")).toBeVisible();
 await page.getByTestId("co-preview-route-first").click();await expect(page.getByRole("dialog")).toContainText("任务路线");await page.getByRole("button",{name:"关闭",exact:true}).click();
 await page.getByTestId("co-plan-first").click();await expect(page.getByTestId("co-task-map")).toBeVisible();
 const before=(await state(page)).topics.length;
 await page.getByTestId("co-task-discuss-login").click();await expect(page.getByTestId("co-message")).toBeVisible();
 expect((await state(page)).topics.length).toBe(before+1);await page.getByTestId("co-message").fill("本任务独立草稿");
 await page.getByRole("button",{name:"任务路线",exact:true}).click();await page.getByTestId("co-task-discuss-login").click();
 expect((await state(page)).topics.length).toBe(before+1);await expect(page.getByTestId("co-message")).toHaveValue("本任务独立草稿");
 await expect(page.locator(".judex-co-chat-sidebar")).not.toContainText("交接信箱");await expect(page.locator(".judex-co-chat-sidebar")).not.toContainText("跨计划讨论");
 await page.screenshot({path:info.outputPath("task-chat.png"),fullPage:true});expect(errors).toEqual([]);
});
test("linked conversation appears in both plans with one identity and one message history",async({page})=>{
 await page.goto("/demo.html#chat/launch/plan/first/main-first");await page.getByTestId("co-message").waitFor();
 await page.getByTestId("co-new-topic").click();await page.getByTestId("co-topic-title").fill("跨计划证据核对");
 await page.getByTestId("co-link-plan-pilot").click();await page.getByTestId("co-link-task-login").click();await page.getByTestId("co-link-task-interview").click();
 await page.getByTestId("co-confirm").click();await expect(page.getByRole("dialog")).toHaveCount(0);
 const s=await state(page),topic=s.topics.find((t:any)=>t.title.zh==="跨计划证据核对");expect(topic.planIds).toEqual(expect.arrayContaining(["first","pilot"]));
 await page.getByTestId("co-message").fill("同一份依据，共同协商");await page.getByTestId("co-send").click();
 await page.goto("/demo.html#chat/launch/plan/pilot/"+topic.id);await expect(page.getByTestId("co-thread")).toContainText("同一份依据，共同协商");
 expect((await state(page)).topics.filter((t:any)=>t.id===topic.id)).toHaveLength(1);
});
test("reporting does not create chats; question requires a human choice and preserves the original",async({page})=>{
 await page.goto("/demo.html#chat/launch/plan/first/main-first");await page.getByTestId("co-message").waitFor();await person(page,"宋雅");
 await page.goto("/demo.html#route/launch/first");await page.getByTestId("co-task-discuss-guide").click();
 const before=(await state(page)).topics.length;
 await page.getByRole("button",{name:"上报进展",exact:true}).click();await page.getByTestId("co-report-body").fill("说明整理中");await page.getByTestId("co-confirm").click();
 expect((await state(page)).topics.length).toBe(before);
 await page.getByRole("button",{name:"提出问题",exact:true}).click();await page.getByTestId("co-report-body").fill("需要与试点协商说明范围");await page.getByTestId("co-confirm").click();
 expect((await state(page)).topics.length).toBe(before);
 const original=(await state(page)).activities.at(-1).id;
 await page.getByRole("button",{name:"单独讨论",exact:true}).click();await page.getByTestId("co-confirm").click();
 expect((await state(page)).topics.length).toBe(before+1);expect((await state(page)).activities.some((r:any)=>r.id===original)).toBe(true);
 await expect(page.getByRole("button",{name:/查看原记录/}).first()).toBeVisible();
});
test("receipt unlocks the next task without accepting the source; approvals activate drafts only after all actors",async({page})=>{
 await page.goto("/demo.html#hub/launch/handoffs");await person(page,"杜衡");
 await page.getByTestId("co-handoff-handoff-login").getByRole("button",{name:"查看并决定",exact:true}).click();await page.getByTestId("co-receive").click();
 let s=await state(page);expect(s.tasks.find((t:any)=>t.id==="login").status).toBe("delivered");expect(s.tasks.find((t:any)=>t.id==="regression").status).toBe("ready");
 await person(page,"林然");await page.getByTestId("co-tab-decisions").click();
 await page.getByTestId("co-decision-arrange-observe").getByRole("button",{name:"查看并决定",exact:true}).click();await page.getByTestId("co-approve").click();
 s=await state(page);expect(s.plans.find((p:any)=>p.id==="observe").status).toBe("draft");
 await person(page,"顾言");await page.getByTestId("co-decision-arrange-observe").getByRole("button",{name:"查看并决定",exact:true}).click();await page.getByTestId("co-approve").click();
 s=await state(page);expect(s.plans.find((p:any)=>p.id==="observe").status).toBe("active");expect(s.tasks.find((t:any)=>t.id==="logs").status).toBe("ready");
});
test("top-right account, project settings, bilingual dark desktop and maximized context",async({page},info)=>{
 await page.goto("/demo.html");await page.getByRole("button",{name:"项目设置 · 轻笺",exact:true}).click();await expect(page.getByRole("dialog")).toContainText("职位与职责");await page.getByRole("button",{name:"关闭",exact:true}).click();
 await page.getByTestId("co-project-launch").click();await page.getByTestId("co-plan-discuss-first").click();await expect(page.getByTestId("co-message")).toBeVisible();
 await page.getByTestId("co-message").fill("保留中央草稿");await page.getByRole("button",{name:"最大化",exact:true}).click();await expect(page.locator(".judex-co-chat-main")).toBeHidden();
 await page.getByRole("button",{name:"恢复三栏",exact:true}).click();await expect(page.getByTestId("co-message")).toHaveValue("保留中央草稿");
 await page.getByTestId("co-account").click();await page.getByRole("menuitem",{name:"切换配色",exact:true}).click();
 await page.getByTestId("co-account").click();await page.getByRole("menuitem",{name:"English",exact:true}).click();await page.setViewportSize({width:1120,height:900});
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await expect(page.getByRole("button",{name:"New discussion",exact:true})).toBeVisible();
 await page.screenshot({path:info.outputPath("dark-en.png"),fullPage:true});
});

test("task and plan acceptance are separate explicit decisions",async({page})=>{
 await page.goto("/demo.html#hub/launch/decisions");
 await page.getByTestId("co-decision-accept-login").getByRole("button",{name:"查看并决定",exact:true}).click();
 await page.getByTestId("co-approve").click();
 let s=await state(page);expect(s.tasks.find((t:any)=>t.id==="login").status).toBe("accepted");expect(s.plans.find((p:any)=>p.id==="first").status).toBe("active");
 expect(s.handoffs.find((h:any)=>h.id==="handoff-login").status).toBe("pending");
 await page.getByTestId("co-decision-accept-foundation").getByRole("button",{name:"查看并决定",exact:true}).click();
 await page.getByTestId("co-approve").click();s=await state(page);
 expect(s.plans.find((p:any)=>p.id==="foundation").status).toBe("accepted");
});
