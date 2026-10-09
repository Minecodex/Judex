import {openTaskAction,selectRouteTask} from './workspace-helpers';
import {test,expect} from "@playwright/test";
import {seedWork} from "../../web/src/features/work/seed";
import {ensureDemoMainTopics} from "../../web/src/features/chat/demoCollaboration";
import {chooseValue} from "./workspace-helpers";

test.beforeEach(async({page},info)=>{
 const seed=ensureDemoMainTopics(seedWork());seed.currentUser="顾言";const task=seed.tasks.find(t=>t.id==="build")!;task.status=info.title.includes("D20")||info.title.includes("D21")?"delivered":"working";
 if(info.title.includes("D20")||info.title.includes("D21")){task.seatIds=["lead","maker"];task.files=[{id:"proof",name:"交付证据",text:"已核对",author:"顾言",at:1}];}
 if(info.title.includes("D21")){const position=structuredClone(seed.positions.find(p=>p.id===seed.seats.find(s=>s.id==="maker")!.positionId)!);position.id="second-position";position.name={zh:"测试职责",en:"Test duty"};seed.positions.push(position);seed.seats.push({...structuredClone(seed.seats.find(s=>s.id==="maker")!),id:"second-maker",positionId:position.id});task.seatIds.push("second-maker");}
 await page.addInitScript(s=>{if(!localStorage.getItem("judex.web.preview.v1"))localStorage.setItem("judex.web.preview.v1",JSON.stringify(s));},seed);
});
test("COOP-D19 route history isolates report attachments and restores the correct task draft",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/route?task=guide");await selectRouteTask(page,"build");await openTaskAction(page,"提出问题");await page.getByTestId("collaboration-report-body").fill("BUILD_DRAFT 不应跟随别的任务");await page.getByRole("dialog").last().locator('input[type="file"]').setInputFiles({name:"build-only.txt",mimeType:"text/plain",buffer:Buffer.from("build only")});await expect(page.getByText("build-only.txt",{exact:true})).toBeVisible();
 await page.goBack();await expect(page).toHaveURL(/task=guide/);await expect(page.getByTestId("collaboration-report-body")).toHaveCount(0);await openTaskAction(page,"提出问题");await expect(page.getByTestId("collaboration-report-body")).toHaveValue("");await expect(page.getByText("build-only.txt",{exact:true})).toHaveCount(0);await page.keyboard.press("Escape");
 await selectRouteTask(page,"build");await openTaskAction(page,"提出问题");await expect(page.getByTestId("collaboration-report-body")).toHaveValue("BUILD_DRAFT 不应跟随别的任务");await expect(page.getByText("build-only.txt",{exact:true})).toBeVisible();await page.getByTestId("collaboration-submit-record").click();await expect(page.getByTestId("collaboration-report-body")).toHaveCount(0);const records=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!).taskActivities);expect(records.filter((v:any)=>v.taskId==="build")).toHaveLength(1);expect(records.some((v:any)=>v.taskId==="guide")).toBe(false);
});
test("COOP-D20 handoff defaults to the acting task member instead of the first participant",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/route?task=build");await openTaskAction(page,"整理交接草稿");await expect(page.getByTestId("handoff-sender-build")).toHaveAttribute("data-value","maker");await page.getByTestId("create-handoff-draft").click();await expect(page).toHaveURL(/tab=deliveries&view=handoff/);const s=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!));expect(s.handoffs.at(-1).sources[0].senderSeatId).toBe("maker");
});
test("COOP-D21 several held sender responsibilities require an explicit selection",async({page})=>{
 await page.goto("/projects/leaf/plans/leaf-first/route?task=build");await openTaskAction(page,"整理交接草稿");await expect(page.getByTestId("create-handoff-draft")).toBeDisabled();await chooseValue(page,"handoff-sender-build","second-maker");await page.getByTestId("create-handoff-draft").click();const s=await page.evaluate(()=>JSON.parse(localStorage.getItem("judex.web.preview.v1")!));expect(s.handoffs.at(-1).sources[0].senderSeatId).toBe("second-maker");
});
