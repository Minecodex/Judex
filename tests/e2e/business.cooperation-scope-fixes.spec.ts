import {openTaskAction,selectRouteTask} from './workspace-helpers';
import {test,expect} from "@playwright/test";
import {command,setupCooperation,applyCooperation} from "./cooperation-business-helpers";
import {chooseValue} from "./workspace-helpers";
test.setTimeout(120000);

test("COOP-R3-A cached task history preserves the correct file draft and ignores an old recheck response",async({browser},info)=>{
 const f=await setupCooperation(browser),{ca,cb,a,b,root,task,plan,identity,flow}=f;
 try{
  await applyCooperation(a,b,root,[{operation:"create_task",targetType:"task",clientRef:"other",fields:{title:"另一任务的独立核对",expectedOutput:"OTHER_TASK_ONLY",acceptanceCriteria:"另一任务的完成条件",planId:plan.id,participantIdentityIds:[identity.id],reviewerIdentityId:identity.id,workflowId:flow.id,nodeId:"work"}}]);const other=(await command(a,"get",root+"/tasks")).items.find((v:any)=>v.title==="另一任务的独立核对");
  for(const item of [task,other])await command(a,"post",root+`/tasks/${item.id}/start`,{expectedVersion:item.version,identityId:identity.id});
  await a.goto(`${root}/plans/${plan.id}/route?task=${other.id}`);await selectRouteTask(a,task.id);await openTaskAction(a,"提出问题");await a.getByTestId("collaboration-report-body").fill("ORIGINAL_TASK_FILE_DRAFT");await a.getByRole("dialog").last().locator('input[type="file"]').setInputFiles({name:"original-task-only.txt",mimeType:"text/plain",buffer:Buffer.from("original task evidence")});await expect(a.getByText("original-task-only.txt",{exact:true})).toBeVisible();
  await a.goBack();await expect(a).toHaveURL(new RegExp(`task=${other.id}`));await expect(a.getByTestId("collaboration-report-body")).toHaveCount(0);await openTaskAction(a,"提出问题");await expect(a.getByTestId("collaboration-report-body")).toHaveValue("");await expect(a.getByText("original-task-only.txt",{exact:true})).toHaveCount(0);await a.keyboard.press("Escape");
  await selectRouteTask(a,task.id);await openTaskAction(a,"提出问题");await expect(a.getByTestId("collaboration-report-body")).toHaveValue("ORIGINAL_TASK_FILE_DRAFT");await expect(a.getByText("original-task-only.txt",{exact:true})).toBeVisible();await a.getByTestId("collaboration-submit-record").click();await expect(a.getByTestId("collaboration-report-body")).toHaveCount(0);expect((await command(a,"get",root+`/tasks/${other.id}/activity`)).items).toHaveLength(0);
  const original=(await command(a,"get",root+`/tasks/${task.id}/activity`)).items;expect(original).toHaveLength(1);expect(original[0].materials[0].name).toBe("original-task-only.txt");
  await openTaskAction(a,"上报进展");await a.getByTestId("collaboration-report-body").fill("DELAYED_RECHECK_DRAFT");await chooseValue(a,"collaboration-input-kind","progress");
  for(const item of [task,other]){const current=await command(a,"get",root+`/tasks/${item.id}`);await command(a,"post",root+`/tasks/${item.id}/reports`,{kind:"progress",text:"A parallel session updated the task",identityId:identity.id,expectedTaskVersion:current.version});}
  await a.reload();await openTaskAction(a,"上报进展");await expect(a.getByTestId("report-stale-warning")).toBeVisible();
  let release!:()=>void,observed!:()=>void,completed!:()=>void,cancelExpected=false;const pause=new Promise<void>(r=>release=r),readStarted=new Promise<void>(r=>observed=r),done=new Promise<void>(r=>completed=r);
  await a.route(`**/api/v1${root}/tasks/${task.id}`,async route=>{const response=await route.fetch();observed();await pause;try{await route.fulfill({response});}catch(error){if(!cancelExpected)throw error;}finally{completed();}});
  await a.getByTestId("report-recheck").click();await readStarted;cancelExpected=true;await chooseValue(a,"report-task-select",other.id);release();await done;await expect(a.getByTestId("report-task-review")).toHaveCount(0);await expect(a.getByTestId("report-confirm-recheck")).toHaveCount(0);await expect(a.getByTestId("report-task-select")).toHaveAttribute("data-value",other.id);await expect(a.getByTestId("collaboration-report-body")).toHaveValue("DELAYED_RECHECK_DRAFT");await a.screenshot({path:info.outputPath("old-response-ignored.png"),fullPage:true});
 }finally{await ca.close();await cb.close();}
});

test("COOP-R3-B explicit source responsibility controls sending and attachment revisions for a member holding several duties",async({browser},info)=>{
 const f=await setupCooperation(browser,true),{ca,cb,a,b,root,task,identity,second,foreign}=f;
 try{
  let current=await command(a,"get",root+`/tasks/${task.id}`);await command(a,"post",root+`/tasks/${task.id}/start`,{expectedVersion:current.version,identityId:second.id});
  for(const [page,role] of [[a,identity],[b,foreign]] as const){current=await command(page,"get",root+`/tasks/${task.id}`);await command(page,"post",root+`/tasks/${task.id}/reports`,{kind:"progress",text:"Required role contributed",identityId:role.id,expectedTaskVersion:current.version});}
  current=await command(a,"get",root+`/tasks/${task.id}`);await command(a,"post",root+`/tasks/${task.id}/reports`,{kind:"delivery",text:"R3_ORIGINAL_DELIVERY",identityId:second.id,expectedTaskVersion:current.version});
  await a.goto(`${root}/plans/${f.plan.id}/route?task=${task.id}`);await openTaskAction(a,"整理交接草稿");await expect(a.getByTestId("create-handoff-draft")).toBeDisabled();await chooseValue(a,"handoff-sender-"+task.id,second.id);await chooseValue(a,"handoff-receiver",foreign.id);await a.getByTestId("create-handoff-draft").click();await expect(a).toHaveURL(/tab=deliveries&view=handoff/);
  const handoff=(await command(a,"get",root+"/handoffs")).items[0],source=handoff.sources[0];expect(source.senderIdentityId).toBe(second.id);await a.getByTestId("send-"+source.id).click();await expect.poll(async()=>(await command(a,"get",root+"/handoffs")).items[0].sources[0].state).toBe("pending");
  await b.goto(`${root}?tab=decisions&view=handoff&item=${handoff.id}`);await b.getByTestId("reject-"+source.id).click();await b.getByTestId("work-reason").fill("补充文件核对");await b.getByTestId("confirm-work-reason").click();await expect.poll(async()=>(await command(a,"get",root+"/handoffs")).items[0].sources[0].state).toBe("rejected");
  await a.reload();await a.getByTestId("revise-"+source.id).click();await a.getByTestId("report-body").fill("R3_ATTACHMENT_REVISION");await a.getByRole("dialog").last().locator('input[type="file"]').setInputFiles({name:"revision-proof.txt",mimeType:"text/plain",buffer:Buffer.from("revision proof")});await expect(a.getByTestId("source-report-identity")).toHaveAttribute("data-value",second.id);
  const report=a.waitForResponse(r=>r.url().endsWith(`/tasks/${task.id}/reports`)&&r.request().method()==="POST");await a.getByTestId("submit-report").click();const response=await report;expect(response.ok()).toBe(true);expect(response.request().postDataJSON().identityId).toBe(second.id);await expect.poll(async()=>(await command(a,"get",root+"/handoffs")).items[0].sources[0].state).toBe("pending");
  await b.reload();await b.getByTestId("receive-"+source.id).click();await expect.poll(async()=>(await command(a,"get",root+"/handoffs")).items[0].sources[0].state).toBe("accepted");expect((await command(a,"get",root+`/tasks/${task.id}`)).status).toBe("delivered");expect((await command(a,"get",root+`/tasks/${task.id}/reports`)).items.some((v:any)=>v.text==="R3_ORIGINAL_DELIVERY")).toBe(true);await a.screenshot({path:info.outputPath("explicit-source-duty.png"),fullPage:true});
 }finally{await ca.close();await cb.close();}
});
