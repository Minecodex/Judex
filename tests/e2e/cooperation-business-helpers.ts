import {expect,type Browser,type Page} from "@playwright/test";
export async function registerMember(page:Page,name:string){
 const email=`member-${crypto.randomUUID()}@cooperation.test`;
 await page.goto("/register");await page.getByLabel(/名称|Name/).fill(name);await page.getByLabel(/邮箱|Email/).fill(email);await page.getByLabel(/密码|Password/).first().fill("cooperation-password-123");await page.getByLabel(/确认密码|Confirm/).fill("cooperation-password-123");await page.getByRole("button",{name:/创建账号|Create account/}).click();await expect(page.getByTestId("workspace-new-project")).toBeVisible();return email;
}
export async function command(page:Page,method:"get"|"post"|"put",path:string,body?:unknown,key?:string){
 const session=(await(await page.request.get("/api/v1/auth/session")).json()).data;
 const response=await page.request.fetch("/api/v1"+path,{method:method.toUpperCase(),data:body,headers:method==="get"?{}:{"X-CSRF-Token":session.csrfToken,"Idempotency-Key":key??crypto.randomUUID()}});
 expect(response.ok(),await response.text()).toBeTruthy();return (await response.json()).data;
}
export async function arrange(page:Page,projectId:string,origin:string,identity:string,workflowId:string,title="首版上线"){
 const root=`/projects/${projectId}`;
 const draft=await command(page,"post",root+"/proposals",{kind:"work_arrangement",topicId:origin,changes:[{operation:"create_plan",targetType:"plan",clientRef:"plan",fields:{title,goal:"可核对的交付",acceptanceCriteria:"证据完整",ownerIdentityId:identity,workflowId,sourceTopicId:origin,forkAfterSeq:1}},{operation:"create_task",targetType:"task",clientRef:"task",fields:{title:"登录功能",expectedOutput:"登录实现与测试证据",acceptanceCriteria:"可核对",participantIdentityIds:[identity],reviewerIdentityId:identity,planId:"plan",workflowId,nodeId:"work"}}]});
 const initial=await command(page,"get",root+`/proposals/${draft.id}/review`);await command(page,"post",root+`/proposals/${draft.id}/submit`,{expectedVersion:draft.version,draftHash:initial.reviewHash});
 const review=await command(page,"get",root+`/proposals/${draft.id}/review`);await command(page,"post",root+`/proposals/${draft.id}/decisions`,{decision:"approve",expectedVersion:review.version,reviewId:review.reviewId,reviewHash:review.reviewHash,slotIds:review.slots.filter((s:any)=>s.canDecide).map((s:any)=>s.id),actingBindingVersions:review.slots.filter((s:any)=>s.canDecide&&s.authorityType==="identity").map((s:any)=>({identityId:s.authorityId,bindingVersion:s.bindingVersion}))});
 return {plan:(await command(page,"get",root+"/plans")).items.find((p:any)=>p.title===title),task:(await command(page,"get",root+"/tasks")).items.find((t:any)=>t.title==="登录功能")};
}

export async function approveCooperation(page:Page,root:string,id:string){
 const review=await command(page,"get",root+`/proposals/${id}/review`),slots=review.slots.filter((s:any)=>s.canDecide&&s.state==="pending");
 if(!slots.length)return;
 await command(page,"post",root+`/proposals/${id}/decisions`,{decision:"approve",expectedVersion:review.version,reviewId:review.reviewId,reviewHash:review.reviewHash,slotIds:slots.map((s:any)=>s.id),actingBindingVersions:slots.filter((s:any)=>s.authorityType==="identity").map((s:any)=>({identityId:s.authorityId,bindingVersion:s.bindingVersion}))});
}
export async function applyCooperation(a:Page,b:Page,root:string,changes:unknown[]){
 const draft=await command(a,"post",root+"/proposals",{kind:"work_change",reason:"协同收尾回归",changes}),initial=await command(a,"get",root+`/proposals/${draft.id}/review`);
 await command(a,"post",root+`/proposals/${draft.id}/submit`,{expectedVersion:draft.version,draftHash:initial.reviewHash});await approveCooperation(a,root,draft.id);await approveCooperation(b,root,draft.id);
 expect((await command(a,"get",root+`/proposals/${draft.id}/review`)).status).toBe("approved");
}
export async function setupCooperation(browser:Browser,multiple=false){
 const ca=await browser.newContext({viewport:{width:1440,height:1000}}),cb=await browser.newContext({viewport:{width:1440,height:1000}});
 const a=await ca.newPage(),b=await cb.newPage();a.setDefaultTimeout(12000);b.setDefaultTimeout(12000);await registerMember(a,"协同发送甲");const email=await registerMember(b,"协同发送乙");
 const project=await command(a,"post","/projects",{title:"收尾缺口真实验收"}),root=`/projects/${project.id}`,user=(await command(a,"get","/auth/session")).user;
 const flow=await command(a,"post",root+"/workflows",{name:"核对流程",nodes:[{id:"work",name:"核对",responsibility:"核对当前任务证据，不替人验收",allowedPositionIds:[],defaultApprovalPolicy:"all"}],advisoryEdges:[]}),version=(await command(a,"get",root+`/workflows/${flow.id}/versions`)).items[0];
 await command(a,"post",root+`/workflows/${flow.id}/publish`,{expectedVersion:1,draftHash:version.draftHash});
 const role=await command(a,"post",root+"/positions",{name:"实现职责",prompt:"collaboration-e2e-duty：核对当前任务",nodeBindings:[{workflowId:flow.id,nodeId:"work"}]}),identity=await command(a,"post",root+"/identities",{positionId:role.id,userId:user.id});
 const invitation=await command(a,"post",root+"/invitations",{targetEmail:email,positionIds:[role.id]}),offer=(await command(b,"get","/me/invitations")).items.find((v:any)=>v.id===invitation.id);await command(b,"post",`/invitations/${invitation.id}/accept`,{expectedVersion:offer?.version??1});
 const foreign=(await command(a,"get",root+"/identities")).items.find((v:any)=>v.templateId===role.id&&v.currentBinding?.userId!==user.id);
 const origin=await command(a,"post",root+"/topics",{title:"安排来源",initialMessage:"明确交付和职责"}),{plan,task}=await arrange(a,project.id,origin.id,identity.id,flow.id);
 let second:any;
 if(multiple){
  const role2=await command(a,"post",root+"/positions",{name:"测试职责",prompt:"collaboration-e2e-duty：核对测试",nodeBindings:[{workflowId:flow.id,nodeId:"work"}]});second=await command(a,"post",root+"/identities",{positionId:role2.id,userId:user.id});
  await applyCooperation(a,b,root,[{operation:"set_assignment",targetType:"task",targetId:task.id,expectedVersion:task.version,fields:{participantIdentityIds:[identity.id,second.id,foreign.id],reviewerIdentityId:identity.id}}]);
 }
 return {ca,cb,a,b,root,project,flow,identity,foreign,second,plan,task};
}
