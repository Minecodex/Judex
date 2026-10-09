import {openTaskAction,selectRouteTask} from './workspace-helpers';
import {test,expect} from "@playwright/test";
import {seedWork} from "../../web/src/features/work/seed";
import {ensureDemoMainTopics} from "../../web/src/features/chat/demoCollaboration";
import {chooseValue,selectPerson} from "./workspace-helpers";

test.beforeEach(async({page},info)=>{
 const seed=ensureDemoMainTopics(seedWork());seed.currentUser="顾言";
 seed.tasks.find(t=>t.id==="build")!.status=info.title.includes("D16")?"ready":"working";
 if(info.title.includes("D16")){
  const position=structuredClone(seed.positions.find(v=>v.id===seed.seats.find(s=>s.id==="maker")!.positionId)!);position.id="second-duty-position";position.name={zh:"核对伙伴",en:"Verification partner"};seed.positions.push(position);
  seed.seats.push({...structuredClone(seed.seats.find(s=>s.id==="maker")!),id:"second-duty",positionId:position.id});seed.tasks.find(t=>t.id==="build")!.seatIds.push("second-duty");
 }
 if(info.title.includes("D17")){const h=seed.handoffs.find(h=>h.id==="first-review")!;h.sources[0].status="accepted";h.sources[1].status="rejected";}
 if(info.title.includes("D18"))seed.topics.push({id:"other-plan-existing",projectId:"leaf",title:{zh:"另一计划的既有核对",en:"Existing review in another plan"},kind:"discussion",planIds:["leaf-next"],taskIds:[],linksVersion:1,messages:[]});
 await page.addInitScript(s=>{if(!localStorage.getItem("judex.web.preview.v1"))localStorage.setItem("judex.web.preview.v1",JSON.stringify(s));},seed);
});

test("COOP-D15 stale report explicitly reviews the latest agreement while keeping its draft",async({page},info)=>{
 await page.goto("/projects/leaf/plans/leaf-first/route?task=build");await openTaskAction(page,"上报进展");await page.getByTestId("collaboration-report-body").fill("保留的旧版本进展草稿");
 await page.keyboard.press("Escape");await page.evaluate(()=>{const s=JSON.parse(localStorage.getItem("judex.web.preview.v1")!);const task=s.tasks.find((t:any)=>t.id==="build");task.revision++;task.criteria=[{zh:"新增的核对要求",en:"New verification requirement"}];localStorage.setItem("judex.web.preview.v1",JSON.stringify(s));});
 await page.reload();await openTaskAction(page,"上报进展");await expect(page.getByTestId("report-stale-warning")).toBeVisible();await expect(page.getByTestId("collaboration-submit-record")).toBeDisabled();
 await page.getByTestId("report-recheck").click();await expect(page.getByTestId("report-task-review")).toContainText("新增的核对要求");await expect(page.getByTestId("collaboration-report-body")).toHaveValue("保留的旧版本进展草稿");await page.screenshot({path:info.outputPath("report-recheck.png"),fullPage:true});
 await page.getByTestId("report-confirm-recheck").click();await page.getByTestId("collaboration-submit-record").click();await expect(page.getByTestId("collaboration-report-body")).toHaveCount(0);const s=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!));expect(s.taskActivities.filter((a:any)=>a.taskId==="build")).toHaveLength(1);expect(s.tasks.find((t:any)=>t.id==="build").revision).toBe(3);
});

test("COOP-D16 multiple task responsibilities require an explicit current identity before starting",async({page},info)=>{
 await page.goto("/projects/leaf/plans/leaf-first/route?task=build");await page.getByTestId("start-task").click();await expect(page.getByTestId("confirm-start-task")).toBeDisabled();await chooseValue(page,"start-task-identity","second-duty");await page.screenshot({path:info.outputPath("multiple-responsibility-start.png"),fullPage:true});await page.getByTestId("confirm-start-task").click();await expect(page.getByTestId("start-task")).toHaveCount(0);const s=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!));expect(s.tasks.find((t:any)=>t.id==="build").status).toBe("working");
});

test("COOP-D17 personal delivery filters follow the actual sender of the outstanding source",async({page})=>{
 await page.goto("/projects/leaf?tab=deliveries");await page.getByRole("button",{name:"需我补充",exact:true}).click();await expect(page.getByTestId("delivery-card-first-review")).toHaveCount(0);await selectPerson(page,"夏禾");await page.getByRole("button",{name:"需我补充",exact:true}).click();await expect(page.getByTestId("delivery-card-first-review")).toBeVisible();
 await page.evaluate(()=>{const s=JSON.parse(localStorage.getItem("judex.web.preview.v1")!);s.currentUser="顾言";s.handoffs.find((h:any)=>h.id==="first-review").sources[1].status="pending";localStorage.setItem("judex.web.preview.v1",JSON.stringify(s));});await page.reload();await page.getByRole("button",{name:"等待对方",exact:true}).click();await expect(page.getByTestId("delivery-card-first-review")).toHaveCount(0);
});

test("COOP-D18 a suggested record explicitly selects an existing discussion in another plan",async({page},info)=>{
 await page.goto("/projects/leaf/plans/leaf-first/route");await page.getByTestId("task-discuss-build").click();await openTaskAction(page,"提出问题");await page.getByTestId("collaboration-report-body").fill("需要关联另一计划已有核对");await page.getByTestId("collaboration-submit-record").click();
 await page.getByRole("button",{name:"选择已有讨论",exact:true}).click();await page.getByTestId("suggestion-topic-search").fill("另一计划");await chooseValue(page,"suggestion-topic-select","other-plan-existing");await expect(page.getByTestId("collaboration-resolve-suggestion")).toBeEnabled();await page.screenshot({path:info.outputPath("cross-scope-existing-discussion.png"),fullPage:true});await page.getByTestId("collaboration-resolve-suggestion").click();await expect(page).toHaveURL(/other-plan-existing/);
 const s=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!)),topic=s.topics.find((t:any)=>t.id==="other-plan-existing");expect(topic.taskIds).toContain("build");expect(topic.planIds).toEqual(expect.arrayContaining(["leaf-first","leaf-next"]));expect(topic.linksVersion).toBe(2);expect(s.discussionSuggestions[0].state).toBe("handled");
});
