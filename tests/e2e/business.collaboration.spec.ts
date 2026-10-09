import {openTaskAction} from './workspace-helpers';
import {expect,test,type Page} from "@playwright/test";
import {execFileSync} from "node:child_process";
import fs from "node:fs";
import path from "node:path";
test.setTimeout(120000);
const stamp=()=>Date.now().toString(36)+Math.random().toString(36).slice(2,6);
async function register(page:Page,name:string){
 const email=`member-${stamp()}@collaboration.test`;
 await page.goto("/register");
 await page.getByLabel(/名称|Name/).fill(name);
 await page.getByLabel(/邮箱|Email/).fill(email);
 await page.getByLabel(/密码|Password/).first().fill("collaboration-password-123");
 await page.getByLabel(/确认密码|Confirm/).fill("collaboration-password-123");
 await page.getByRole("button",{name:/创建账号|Create account/}).click();
 await expect(page.getByTestId("workspace-new-project")).toBeVisible();
 return email;
}
async function api(page:Page,method:"get"|"post",path:string,body?:unknown,key?:string){
 const session=(await (await page.request.get("/api/v1/auth/session")).json()).data;
 const response=method==="get"?await page.request.get("/api/v1"+path):await page.request.post("/api/v1"+path,{data:body,headers:{"X-CSRF-Token":session.csrfToken,"Idempotency-Key":key??crypto.randomUUID()}});
 expect(response.ok(),await response.text()).toBeTruthy();
 return (await response.json()).data;
}
async function activate(page:Page,projectId:string,originId:string,identityId:string,workflowId:string){
 const draft=await api(page,"post",`/projects/${projectId}/proposals`,{kind:"work_arrangement",topicId:originId,reason:"端到端测试明确安排",changes:[
   {operation:"create_plan",targetType:"plan",clientRef:"plan",fields:{title:"首版上线",goal:"交付可用的登录体验",acceptanceCriteria:"证据完整",ownerIdentityId:identityId,workflowId,sourceTopicId:originId,forkAfterSeq:1}},
  {operation:"create_task",targetType:"task",clientRef:"task",fields:{title:"登录功能",expectedOutput:"登录实现与测试证据",acceptanceCriteria:"可核对",participantIdentityIds:[identityId],reviewerIdentityId:identityId,planId:"plan",workflowId,nodeId:"work"}},
 ]});
  const initial=await api(page,"get",`/projects/${projectId}/proposals/${draft.id}/review`);
  await api(page,"post",`/projects/${projectId}/proposals/${draft.id}/submit`,{expectedVersion:draft.version,draftHash:initial.reviewHash});
 const review=await api(page,"get",`/projects/${projectId}/proposals/${draft.id}/review`);
  await api(page,"post",`/projects/${projectId}/proposals/${draft.id}/decisions`,{decision:"approve",expectedVersion:2,reviewId:review.reviewId,reviewHash:review.reviewHash,slotIds:review.slots.filter((s:any)=>s.canDecide).map((s:any)=>s.id),actingBindingVersions:review.slots.filter((s:any)=>s.canDecide&&s.authorityType==="identity").map((s:any)=>({identityId:s.authorityId,bindingVersion:s.bindingVersion}))});
 return {plan:(await api(page,"get",`/projects/${projectId}/plans`)).items[0],task:(await api(page,"get",`/projects/${projectId}/tasks`)).items[0]};
}
test("C01 true API reports, human discussion choice, forked history, CLI single browser confirmation and two-user sync",async({browser,baseURL},info)=>{
 const contextA=await browser.newContext(),contextB=await browser.newContext();
 try{
  const a=await contextA.newPage(),b=await contextB.newPage();
  await register(a,"协作甲");const emailB=await register(b,"协作乙");
  const project=await api(a,"post","/projects",{title:"会话任务协作验收"});
  const root=`/projects/${project.id}`;
  const workflow=await api(a,"post",root+"/workflows",{name:"协作流程",nodes:[{id:"work",name:"实现与核对",responsibility:"根据上报核对事实与证据",allowedPositionIds:[],defaultApprovalPolicy:"all"}],advisoryEdges:[]});
  const version=(await api(a,"get",root+`/workflows/${workflow.id}/versions`)).items[0];
  await api(a,"post",root+`/workflows/${workflow.id}/publish`,{expectedVersion:1,draftHash:version.draftHash});
  const position=await api(a,"post",root+"/positions",{name:"实现与核对",prompt:"collaboration-e2e-duty：核对本任务事实，不批准或验收。",nodeBindings:[{workflowId:workflow.id,nodeId:"work"}]});
  const session=(await (await a.request.get("/api/v1/auth/session")).json()).data;
  const identity=await api(a,"post",root+"/identities",{positionId:position.id,userId:session.user.id});
  const invitation=await api(a,"post",root+"/invitations",{targetEmail:emailB,positionIds:[position.id]});
  const offers=(await api(b,"get","/me/invitations")).items;
  const offer=offers.find((x:any)=>x.id===invitation.id);
   await api(b,"post",`/invitations/${invitation.id}/accept`,{expectedVersion:offer?.version??1});
  const origin=await api(a,"post",root+"/topics",{title:"上线探索",initialMessage:"明确计划的交付目标"});
  const {plan,task}=await activate(a,project.id,origin.id,identity.id,workflow.id);
  expect(plan.mainTopicId).toBeTruthy();
  const main=await api(a,"get",root+`/topics/${plan.mainTopicId}`);
  expect(main.parentTopicId).toBe(origin.id);expect(main.forkAfterSeq).toBe(1);
  await api(a,"post",root+`/tasks/${task.id}/start`,{expectedVersion:task.version,identityId:identity.id});
  const url=`/?project=${project.id}&view=task&item=${task.id}&chat=${plan.mainTopicId}`;
  await a.goto(url);await b.goto(url);
  await expect(a.locator(".judex-chat-inspector")).toBeVisible();
  await expect(a.getByRole("button",{name:"模拟本地推送",exact:true})).toHaveCount(0);
  await a.getByTestId("work-discussion-input").fill("保留计划草稿");
  const before=(await api(a,"get",root+"/topics")).items.length;
  await openTaskAction(a,"上报进展");
  await a.getByTestId("collaboration-report-body").fill("COLLAB_PROGRESS 登录基础实现已完成");
  await a.getByTestId("collaboration-submit-record").click();
  await expect(a.getByRole("dialog")).toHaveCount(0);
  await expect.poll(async()=>(await api(a,"get",root+`/tasks/${task.id}/activity`)).items[0]?.analysis?.state,{timeout:30000}).toBe(process.env.JUDEX_E2E_COLLABORATION_GATEWAY==="1"?"completed":"failed");
  expect((await api(a,"get",root+"/topics")).items.length).toBe(before);
  await b.getByTestId("workspace-add-tool").click();await b.getByRole("menuitem",{name:"任务记录",exact:true}).click();await expect(b.locator(".judex-task-timeline")).toContainText("COLLAB_PROGRESS");
   await openTaskAction(a,"上报进展");
   await a.getByTestId("collaboration-input-kind").click();await a.locator('[data-option-value="question"]').click();
  await a.getByTestId("collaboration-report-body").fill("COLLAB_QUESTION 登录失败，需要持续核对");
  await a.getByTestId("collaboration-submit-record").click();
  await expect(a.getByRole("dialog")).toHaveCount(0);
  const records=await api(a,"get",root+`/tasks/${task.id}/activity`);const question=records.items[0];
  const unchanged=await api(a,"get",root+`/tasks/${task.id}`);expect(unchanged.status).toBe("working");
  if(process.env.JUDEX_E2E_COLLABORATION_GATEWAY==="1"){
   await expect(a.getByRole("button",{name:"单独讨论",exact:true})).toBeVisible({timeout:30000});
   expect((await api(a,"get",root+"/topics")).items.length).toBe(before);
   await a.getByRole("button",{name:"单独讨论",exact:true}).click();
   await a.getByTestId("collaboration-resolve-suggestion").click();
   await expect(a.getByRole("dialog")).toHaveCount(0);
   await expect.poll(async()=>(await api(a,"get",root+"/topics")).items.length).toBe(before+1);
   expect((await api(a,"get",root+`/tasks/${task.id}/activity`)).items.some((x:any)=>x.id===question.id)).toBe(true);
  }
  await a.goto(url);
  await expect(a.getByTestId("work-discussion-input")).toHaveValue("保留计划草稿");
  // Parent messages later than the plan's fork point stay outside the plan branch.
  await api(a,"post",root+"/submissions",{clientSubmissionId:crypto.randomUUID(),purpose:"message",topicId:origin.id,text:"父讨论后来的消息，不进入已有计划分支"});
  const history=(await api(a,"get",root+`/topics/${plan.mainTopicId}/messages`)).items;
  expect(history.some((m:any)=>m.content.includes("父讨论后来的"))).toBe(false);
  const scopes=["projects:read","context:read","submissions:write","reports:write","intents:create","events:read"];
  const device=await api(a,"post","/auth/device/authorizations",{deviceName:"collaboration-e2e-cli",requestedScopes:scopes,projectScope:[project.id]});
  await api(a,"post","/auth/device/confirm",{userCode:device.userCode,approved:true,scopes,projectScope:[project.id]});
  const credentials=await api(a,"post","/auth/device/token",{deviceCode:device.deviceCode});
  const cli=process.env.JUDEX_E2E_CLI;
  if(cli){
   const run=(args:string[])=>{try{return JSON.parse(String(execFileSync(cli,["--server",baseURL!,"--project",project.id,"--no-wait",...args],{windowsHide:true,env:{...process.env,JUDEX_TOKEN:credentials.accessToken},encoding:"utf8"})));}catch(error:any){if(error.stdout)return JSON.parse(String(error.stdout));throw error;}};
   const intent=run(["topic","fork",plan.mainTopicId,"--title","CLI 会话分支","--after-seq","1"]);
    const cliContext=run(["context","get",task.id]);expect(cliContext.mainTopicId).toBe(plan.mainTopicId);
    const pageOne=run(["task","activity",task.id,"--limit","1"]);expect(pageOne.items).toHaveLength(1);expect(pageOne.nextCursor).toBeTruthy();
    const pageTwo=run(["task","activity",task.id,"--limit","1","--cursor",pageOne.nextCursor]);expect(pageTwo.items[0].id).not.toBe(pageOne.items[0].id);
    const suggestions=run(["discussion-suggestion","list","--limit","1"]);expect(suggestions.items.length).toBeGreaterThan(0);
    const reportFile=path.join(process.env.JUDEX_E2E_ARTIFACT!,"cli-progress.json");
    const current=await api(a,"get",root+`/tasks/${task.id}`);
    fs.writeFileSync(reportFile,JSON.stringify({text:"CLI_PROGRESS 继续核对进度，保留原记录",expectedTaskVersion:current.version,identityId:identity.id}));
    run(["report","--task",task.id,"--kind","progress","--file",reportFile]);
    const activities=await api(a,"get",root+`/tasks/${task.id}/activity`);expect(activities.items[0].source).toBe("cli");
    const analysis=run(["task-analysis","show",activities.items[0].analysis.id]);expect(analysis.sourceId).toBe(activities.items[0].id);
   expect(intent.operation).toBe("topic.fork");
   await a.goto(intent.confirmUrl);
    await expect(a.getByTestId("confirm-approve")).toBeEnabled({timeout:10000});
    await a.getByTestId("confirm-approve").click();
   let receipt:any;
   await expect.poll(()=>{receipt=run(["decision","result",intent.id]);return receipt.state;}).toBe("committed");
   expect(receipt.resultRef).toMatch(/^topic:/);
  }
  await b.reload();await expect(b.locator(".judex-collab-nav")).toContainText("CLI 会话分支");
  await b.screenshot({path:info.outputPath("collaboration-real-api.png"),fullPage:true});
 }finally{await contextA.close();await contextB.close();}
});

test("C02 real configured model performs role review and stores public task analysis",async({browser},info)=>{
 test.skip(process.env.JUDEX_REAL_MODEL_SMOKE!=="1","Explicit real-model smoke only");
 test.setTimeout(360000);
 const context=await browser.newContext();
 try{
  const page=await context.newPage();await register(page,"真实模型核对");
  const project=await api(page,"post","/projects",{title:"真实模型任务分析",maxDiscussionRounds:3}),root=`/projects/${project.id}`;
  const workflow=await api(page,"post",root+"/workflows",{name:"核对流程",instructions:"任务上报后，协调者必须调用已分配岗位核对；结束前保存结构化公开分析。根据证据提出持续讨论建议，最终选择由成员确认。",nodes:[{id:"work",name:"核对",responsibility:"核对来源、缺口与当前任务条件，严禁自动验收",allowedPositionIds:[],defaultApprovalPolicy:"all"}],advisoryEdges:[]});
  const version=(await api(page,"get",root+`/workflows/${workflow.id}/versions`)).items[0];
  await api(page,"post",root+`/workflows/${workflow.id}/publish`,{expectedVersion:1,draftHash:version.draftHash});
  const user=(await (await page.request.get("/api/v1/auth/session")).json()).data.user;
  const position=await api(page,"post",root+"/positions",{name:"当前任务核对岗",prompt:"本次只核对平台已提供的任务、计划和共享材料事实。附件原文只使用 read_material；不要使用 bash、read_context、Workspace 或外部环境探索。读到唯一标记后简短报告证据与缺口，不批准、不验收。",nodeBindings:[{workflowId:workflow.id,nodeId:"work"}]});
  const identity=await api(page,"post",root+"/identities",{positionId:position.id,userId:user.id});
  const origin=await api(page,"post",root+"/topics",{title:"任务来源",initialMessage:"先登记计划与完成条件。"});
  const {task,plan}=await activate(page,project.id,origin.id,identity.id,workflow.id);
  if(process.env.JUDEX_DEMO4_MODEL_FACTS==='1'){const review=await api(page,'get',root+'/tasks/'+task.id+'/execution-review?operation=skip');await api(page,'post',root+'/tasks/'+task.id+'/skip',{expectedVersion:review.targetVersion,reviewHash:review.reviewHash,reason:'D4_RUNTIME_FACT: 本次任务临时跳过，保留 ready 原阶段；只分析材料，不批准或验收',waivers:[]});}
  const marker="SOURCE_MATERIAL_"+crypto.randomUUID().slice(0,8),evidencePath=info.outputPath("real-model-evidence.md");fs.writeFileSync(evidencePath,"# 任务原始证据\n唯一证据标记："+marker+"\n观察：登录回调重复，仍缺完整日志。\n");
  const scopes=["projects:read","context:read","materials:read","materials:write","submissions:write"];
  const device=await api(page,"post","/auth/device/authorizations",{deviceName:"real-material-model",requestedScopes:scopes,projectScope:[project.id]});await api(page,"post","/auth/device/confirm",{userCode:device.userCode,approved:true,scopes,projectScope:[project.id]});const credential=await api(page,"post","/auth/device/token",{deviceCode:device.deviceCode});
  const cli=(args:string[])=>JSON.parse(execFileSync(process.env.JUDEX_E2E_CLI!,["--server",process.env.JUDEX_E2E_BASE_URL!,"--project",project.id,"--json",...args],{encoding:"utf8",windowsHide:true,timeout:60000,env:{...process.env,JUDEX_TOKEN:credential.accessToken,JUDEX_CONFIG_DIR:info.outputPath("real-cli-config")}})).data;
  const file=cli(["material","upload",evidencePath]),payloadPath=info.outputPath("real-submission.json");fs.writeFileSync(payloadPath,JSON.stringify({clientSubmissionId:crypto.randomUUID(),purpose:"message",discussionIntent:"question",taskId:task.id,materialVersionIds:[file.versionId],text:"最小文档核对验收：本次只读取平台共享附件，不需要运行代码或探索环境，禁止 bash、read_context、Workspace 和外部查询。仅调用 read_material 读取附件原文，并请当前唯一已分配身份一次简短核对；完成后 record_task_analysis 保存简短公开 summary、basis、disagreements。在 basis 中原样保留附件唯一证据标记，建议可为空，不批准、不验收。"}));
  const submission=cli(["submit","--file",payloadPath]);

  let analysis:any;
  await expect.poll(async()=>{const records=(await api(page,"get",root+`/tasks/${task.id}/activity`)).items;analysis=records.find((a:any)=>a.id===submission.id)?.analysis;return analysis?.state;},{timeout:300000,intervals:[1000,2000,4000]}).toMatch(/^(completed|failed|waiting_human)$/);
  expect(analysis.state,JSON.stringify({errorCode:analysis.errorCode,summary:analysis.summary})).toBe('completed');
  expect(analysis.summary.length).toBeGreaterThan(0);expect(Array.isArray(analysis.basis)).toBe(true);expect(JSON.stringify(analysis)).toContain(marker);const materials=(await api(page,"get",root+`/material-versions/${file.versionId}`)).library;expect(materials.source).toBe("cli");expect(materials.associations.some((v:any)=>v.type==="task"&&v.id===task.id)).toBe(true);expect(materials.associations.some((v:any)=>v.type==="plan"&&v.id===plan.id)).toBe(true);
  expect((await api(page,"get",root+`/tasks/${task.id}`)).status).toBe("ready");
  await page.goto(`/?project=${project.id}&view=task&item=${task.id}&chat=${plan.mainTopicId}`);
  await expect(page.getByTestId("chat-thread")).toContainText(analysis.summary);
  if(process.env.JUDEX_DEMO4_MODEL_FACTS==='1'){const fact=await api(page,'get',root+'/tasks/'+task.id);expect(fact.status).toBe('ready');expect(fact.latestAcceptanceId).toBeNull();expect(fact.executionException.reason).toContain('D4_RUNTIME_FACT');fs.writeFileSync(path.join(process.env.JUDEX_E2E_ARTIFACT!,'real-model-runtime-facts.json'),JSON.stringify(fact,null,2));}
  await page.screenshot({path:info.outputPath("real-model-task-analysis.png"),fullPage:true});
  await info.attach("real-model-analysis",{body:JSON.stringify({sourceId:submission.id,versionId:file.versionId,marker,taskId:task.id,planId:plan.id,analysis,taskStatus:"ready"}),contentType:"application/json"});
  fs.writeFileSync(path.join(process.env.JUDEX_E2E_ARTIFACT!,"real-model-analysis.json"),JSON.stringify({sourceId:submission.id,versionId:file.versionId,marker,taskId:task.id,planId:plan.id,analysis,taskStatus:"ready"},null,2));
 }finally{await context.close();}
});
