import {openTaskAction,selectRouteTask} from './workspace-helpers';
import {test,expect,type Browser,type Page} from "@playwright/test";
import {registerMember,command,arrange} from "./cooperation-business-helpers";
import {chooseValue} from "./workspace-helpers";
test.setTimeout(180000);

import {applyCooperation as apply,setupCooperation as setup} from "./cooperation-business-helpers";

test("COOP-R2-A true multi-responsibility start and stale report recovery after another member reports",async({browser},info)=>{
 const f=await setup(browser,true),{ca,cb,a,b,root,task,second,foreign}=f;
 try{
  await a.goto(`${root}?tab=deliveries&view=task&item=${task.id}`);await a.getByTestId("start-task").click();await expect(a.getByTestId("confirm-start-task")).toBeDisabled();await chooseValue(a,"start-task-identity",second.id);
  const started=a.waitForResponse(r=>r.url().endsWith(`/tasks/${task.id}/start`)&&r.request().method()==="POST");await a.getByTestId("confirm-start-task").click();expect((await started).request().postDataJSON().identityId).toBe(second.id);await expect(a.getByTestId("start-task")).toHaveCount(0);
  await openTaskAction(a,"上报进展");await chooseValue(a,"report-identity",second.id);await a.getByTestId("collaboration-report-body").fill("PRESERVED_DRAFT_A 保留甲的进展草稿");
  const before=await command(b,"get",root+`/tasks/${task.id}`);await command(b,"post",root+`/tasks/${task.id}/reports`,{kind:"progress",text:"OTHER_MEMBER_PROGRESS 乙先登记进展",identityId:foreign.id,expectedTaskVersion:before.version});
  await a.reload();await openTaskAction(a,"上报进展");await expect(a.getByTestId("collaboration-report-body")).toHaveValue("PRESERVED_DRAFT_A 保留甲的进展草稿");await expect(a.getByTestId("collaboration-submit-record")).toBeDisabled();
  await ca.setOffline(true);await a.getByTestId("report-recheck").click();await expect(a.getByRole("alert").filter({hasText:/网络|Network/}).last()).toBeVisible();await ca.setOffline(false);await a.getByTestId("report-recheck").click();await expect(a.getByTestId("report-task-review")).toContainText("登录实现与测试证据");await a.screenshot({path:info.outputPath("real-stale-report-recheck.png"),fullPage:true});
  await a.getByTestId("report-confirm-recheck").click();const sent=a.waitForResponse(r=>r.url().endsWith(`/tasks/${task.id}/reports`)&&r.request().method()==="POST");await a.getByTestId("collaboration-submit-record").click();const response=await sent;expect(response.ok()).toBe(true);expect(response.request().postDataJSON().identityId).toBe(second.id);await expect(a.getByTestId("collaboration-report-body")).toHaveCount(0);
  const reports=(await command(a,"get",root+`/tasks/${task.id}/reports`)).items;expect(reports).toHaveLength(2);expect(reports.some((v:any)=>v.text.includes("OTHER_MEMBER_PROGRESS"))).toBe(true);expect(reports.some((v:any)=>v.text.includes("PRESERVED_DRAFT_A"))).toBe(true);expect((await command(a,"get",root+`/tasks/${task.id}`)).status).toBe("working");
 }finally{await ca.close();await cb.close();}
});

test("COOP-R2-B AI links a later-page discussion across plans and an old editor must recheck",async({browser},info)=>{
 expect(process.env.JUDEX_E2E_COLLABORATION_GATEWAY).toBe("1");
 const f=await setup(browser),{ca,cb,a,b,root,task,plan}=f;
 try{
  const other=await command(a,"post",root+"/plans",{title:"其他计划"}),topics:any[]=[];for(let i=0;i<55;i++)topics.push(await command(a,"post",root+"/topics",{title:`既有核对 ${i}`,links:[{objectType:"plan",objectId:other.id}]}));const target=topics[0];
  await command(a,"post",root+"/submissions",{clientSubmissionId:crypto.randomUUID(),purpose:"message",discussionIntent:"question",taskId:task.id,text:"COLLAB_QUESTION 核对其他计划的已有依据"});
  await expect.poll(async()=>(await command(a,"get",root+`/discussion-suggestions?taskId=${task.id}`)).items.length,{timeout:30000}).toBe(1);
  let firstTopicRead=true;await b.route(`**/api/v1${root}/topics/${target.id}`,async route=>{if(firstTopicRead){firstTopicRead=false;await new Promise(resolve=>setTimeout(resolve,400));}await route.continue();});await b.goto(`${root}/chat/${target.id}?editor=topic&editTopic=${target.id}`);await expect(b.getByTestId("save-topic-links")).toBeEnabled();await expect(b.getByTestId("discussion-title")).toHaveValue(target.title);
  await a.goto(`${root}/plans/${plan.id}/chat/${plan.mainTopicId}`);await a.getByTestId("work-discussion-input").fill("AI建议处理前的计划草稿");await expect(a.getByTestId("scoped-topic-"+target.id)).toHaveCount(0);
  const firstPage=a.waitForResponse(r=>{const u=new URL(r.url());return u.pathname.endsWith("/topics")&&u.searchParams.has("q")&&!u.searchParams.has("planId")&&!u.searchParams.has("taskId")&&!u.searchParams.has("cursor");});await a.getByRole("button",{name:"选择已有讨论",exact:true}).click();expect((await (await firstPage).json()).data.items.some((v:any)=>v.id===target.id)).toBe(false);
  const nextPage=a.waitForResponse(r=>{const u=new URL(r.url());return u.pathname.endsWith("/topics")&&u.searchParams.has("cursor")&&!u.searchParams.has("planId");});await a.getByRole("dialog").getByRole("button",{name:"加载更多",exact:true}).click();expect((await (await nextPage).json()).data.items.some((v:any)=>v.id===target.id)).toBe(true);
  const searched=a.waitForResponse(r=>{const u=new URL(r.url());return u.pathname.endsWith("/topics")&&u.searchParams.get("q")==="既有核对 0";});await a.getByTestId("suggestion-topic-search").fill("既有核对 0");expect((await (await searched).json()).data.items.map((v:any)=>v.id)).toEqual([target.id]);await chooseValue(a,"suggestion-topic-select",target.id);await a.screenshot({path:info.outputPath("real-cross-plan-picker.png"),fullPage:true});await a.getByTestId("collaboration-resolve-suggestion").click();await expect(a).toHaveURL(new RegExp(target.id));
  const linked=await command(a,"get",root+`/topics/${target.id}`);expect(linked.linksVersion).toBe(target.linksVersion+1);expect(linked.links.some((r:any)=>r.objectType==="task"&&r.objectId===task.id)).toBe(true);
  await b.getByTestId("save-topic-links").click();await expect(b.getByRole("alert")).toContainText("关联");await b.getByTestId("topic-recheck").click();await expect(b.getByTestId("save-topic-links")).toBeEnabled();expect((await command(a,"get",root+`/topics/${target.id}`)).linksVersion).toBe(linked.linksVersion);
  const suggestion=(await command(a,"get",root+`/discussion-suggestions?taskId=${task.id}`)).items[0];await command(a,"post",root+`/discussion-suggestions/${suggestion.id}`,{expectedVersion:1,mode:"link",topicId:target.id});expect((await command(a,"get",root+`/topics/${target.id}`)).linksVersion).toBe(linked.linksVersion);expect((await command(a,"get",root+`/tasks/${task.id}/activity`)).items).toHaveLength(1);
  await a.goto(`${root}/plans/${plan.id}/chat/${plan.mainTopicId}`);await expect(a.getByTestId("work-discussion-input")).toHaveValue("AI建议处理前的计划草稿");await expect(a.getByTestId("scoped-topic-"+target.id)).toBeVisible();
 }finally{await ca.close();await cb.close();}
});

test("COOP-R2-C two senders see only their own waiting and revision sources",async({browser},info)=>{
 const f=await setup(browser),{ca,cb,a,b,root,task,plan,identity,foreign,flow}=f;
 try{
  await apply(a,b,root,[{operation:"create_task",targetType:"task",clientRef:"source-b",fields:{title:"乙的来源任务",expectedOutput:"乙的核对结果",acceptanceCriteria:"可核对",planId:plan.id,participantIdentityIds:[foreign.id],reviewerIdentityId:identity.id,workflowId:flow.id,nodeId:"work"}}]);const sourceB=(await command(a,"get",root+"/tasks")).items.find((t:any)=>t.title==="乙的来源任务");
  let sourceA=await command(a,"get",root+`/tasks/${task.id}`);await command(a,"post",root+`/tasks/${task.id}/reports`,{kind:"delivery",text:"SOURCE_A_REPORT 已完成甲的交付",identityId:identity.id,expectedTaskVersion:sourceA.version});await command(b,"post",root+`/tasks/${sourceB.id}/reports`,{kind:"delivery",text:"SOURCE_B_REPORT 乙的交付待核对",identityId:foreign.id,expectedTaskVersion:sourceB.version});
  const review=await command(a,"get",root+`/tasks/${task.id}/acceptance-review`);await command(a,"post",root+`/tasks/${task.id}/acceptances`,{reviewId:review.reviewId,reviewHash:review.reviewHash,expectedVersion:review.targetVersion,decision:"accept"});
  const created=await command(a,"post",root+"/handoffs",{title:"两个发送者的交接",kind:"stage",targetTaskId:sourceB.id,receiverIdentityId:identity.id,sources:[{sourceTaskId:task.id,senderIdentityId:identity.id},{sourceTaskId:sourceB.id,senderIdentityId:foreign.id}]}),h=(await command(a,"get",root+"/handoffs")).items.find((h:any)=>h.id===created.id),sa=h.sources.find((s:any)=>s.sourceTaskId===task.id),sb=h.sources.find((s:any)=>s.sourceTaskId===sourceB.id);
  await a.goto(`${root}?tab=deliveries&view=handoff&item=${h.id}`);await a.getByTestId("send-"+sa.id).click();await a.getByTestId("receive-"+sa.id).click();await b.goto(`${root}?tab=deliveries&view=handoff&item=${h.id}`);await b.getByTestId("send-"+sb.id).click();
  await a.goto(root+"?tab=deliveries");await a.getByRole("button",{name:"等待对方",exact:true}).click();await expect(a.getByTestId("delivery-card-"+h.id)).toHaveCount(0);expect((await command(a,"get",root+"/deliveries?filter=waiting&type=handoff")).items).toHaveLength(0);
  await a.goto(`${root}?tab=decisions&view=handoff&item=${h.id}`);await a.getByTestId("reject-"+sb.id).click();await a.getByTestId("work-reason").fill("仅乙的来源需要补充");await a.getByTestId("confirm-work-reason").click();await expect.poll(async()=>(await command(a,"get",root+"/handoffs")).items.find((x:any)=>x.id===h.id).sources.find((x:any)=>x.id===sb.id).state).toBe("rejected");
  await a.goto(root+"?tab=deliveries");await a.getByRole("button",{name:"需我补充",exact:true}).click();await expect(a.getByTestId("delivery-card-"+h.id)).toHaveCount(0);expect((await command(a,"get",root+"/deliveries?filter=revise&type=handoff")).items).toHaveLength(0);
  await b.goto(root+"?tab=deliveries");await b.getByRole("button",{name:"需我补充",exact:true}).click();await expect(b.getByTestId("delivery-card-"+h.id)).toBeVisible();expect((await command(a,"get",root+`/tasks/${task.id}`)).status).toBe("accepted");await b.screenshot({path:info.outputPath("real-personal-handoff-filters.png"),fullPage:true});
 }finally{await ca.close();await cb.close();}
});
