// Scene team creation and invitations through actual product pages.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
const root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8'));
const result=JSON.parse(fs.readFileSync(path.join(root,'KA01-project-result.json'),'utf8'));
if(result.state!=='committed'||!/^project:[0-9a-f-]{36}$/.test(result.resultRef))throw new Error('Confirmed Kimi project required');
const projectID=result.resultRef.slice(8),baseURL=`http://127.0.0.1:${env.port}`,prefix='/api/v1/projects/'+projectID;
const evidenceFile=path.join(root,'A-team.json');
const evidence=fs.existsSync(evidenceFile)?JSON.parse(fs.readFileSync(evidenceFile,'utf8')):{projectID,positions:{},invitations:{}};
const save=()=>fs.writeFileSync(evidenceFile,JSON.stringify(evidence,null,2)+'\n');
const browser=await chromium.launch();
const owner=await browser.newContext({baseURL,storageState:actors.U0.statePath,viewport:{width:1440,height:1000}});
const page=await owner.newPage();
const read=async suffix=>(await(await owner.request.get(prefix+suffix)).json()).data;
const choose=async(id,value)=>{await page.getByTestId(id).click();await page.locator('[data-option-value='+JSON.stringify(value)+']').click();await page.getByRole('listbox',{includeHidden:true}).waitFor({state:'detached'});};
try{
 await page.goto('/?project='+projectID+'&settings=team');
 for(const [key,name,prompt] of [
  ['product','产品负责人','确认 MiniIssue 范围、需求、计划与交付依据；保留未完成条件，最终正式安排和计划验收由有权人确认。'],
  ['backend','后端开发','负责 API、数据模型、边界修复、性能优化和隔离恢复。核对接口与真实运行证据，不能以模型结论代人批准或验收。'],
  ['frontend','前端开发','负责桌面 Web、过滤分页、交互与视觉资产集成。核对实际页面与前后端契约，保留异常和待验证项。'],
  ['qa','测试工程师','独立核对接口、桌面交互、缺陷复现、回归、基准和恢复证据。同职位不同任职身份分别出具贡献与意见。'],
  ['ops','构建运维','负责可复现构建、发布包、隔离故障诊断和恢复操作依据。只分析本项目实验，不操作正常业务环境，不代人验收。'],
 ]){
  if(evidence.positions[key])continue;
  const current=(await read('/positions')).items.find(p=>p.name===name);
  if(current){evidence.positions[key]=current.id;save();continue;}
  await page.getByTestId('new-position').click();await page.getByTestId('position-name').fill(name);await page.getByTestId('position-prompt').fill(prompt);
  await choose('position-model',env.models.position);await page.getByTestId('save-position').click();await page.getByRole('dialog').waitFor({state:'detached'});
  const created=(await read('/positions')).items.find(p=>p.name===name);if(!created)throw new Error('Position not persisted: '+key);
  evidence.positions[key]=created.id;save();console.log('Created role '+name+' through UI');
 }
 const product=evidence.positions.product;
 const identities=(await read('/identities')).items;
 if(!identities.some(i=>i.templateId===product&&i.currentBinding?.userId===actors.U0.userId)){
  await page.getByTestId('assign-position-'+product).click();await choose('position-assignment-member',actors.U0.userId);
  await page.getByTestId('assign-existing-position').click();await page.getByRole('dialog').waitFor({state:'detached'});
 }
 for(const [actorID,roles] of [['U1',['backend','ops']],['U2',['frontend']],['U3',['qa']],['U4',['qa']]]){
  if(!evidence.invitations[actorID]){
   await page.getByTestId('next-invite').click();await page.getByTestId('next-invite-name').fill(actors[actorID].email);
   for(const role of roles)await page.getByTestId('invite-position-'+evidence.positions[role]).getByRole('checkbox').press('Space');
   const sent=page.waitForResponse(r=>r.url().endsWith(prefix+'/invitations')&&r.request().method()==='POST');
   await page.getByTestId('next-create-invite').click();const response=await sent;if(!response.ok())throw new Error('Invitation creation failed');
   const invitation=(await response.json()).data;
   const url=new URL(invitation.inviteUrl,baseURL);if(url.origin!==baseURL)throw new Error('Invitation left owned environment');
   evidence.invitations[actorID]={id:invitation.id,url:url.href,roles,accepted:false};save();
   await page.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();await page.getByRole('dialog').waitFor({state:'detached'});
  }
  if(!evidence.invitations[actorID].accepted){
   const member=await browser.newContext({baseURL,storageState:actors[actorID].statePath,viewport:{width:1440,height:1000}});
   try{
    const invitePage=await member.newPage();await invitePage.goto(evidence.invitations[actorID].url);
    await invitePage.getByRole('button',{name:'接受邀请',exact:true}).click();await invitePage.waitForURL(url=>url.pathname==='/');
    await member.storageState({path:actors[actorID].statePath});evidence.invitations[actorID].accepted=true;save();
    console.log(actorID+' accepted assigned roles through UI');
   }finally{await member.close();}
  }
 }
 evidence.identities=(await read('/identities')).items.filter(i=>i.kind==='position');
 if(evidence.identities.length!==6)throw new Error('Expected six distinct initial role identities');
 const qa=evidence.identities.filter(i=>i.templateId===evidence.positions.qa);if(qa.length!==2||qa[0].id===qa[1].id)throw new Error('Same-role holders must remain distinct');
 evidence.createdVia='team_assignment_and_invitation_ui';save();await page.reload();await page.screenshot({path:path.join(root,'A-team-created.png'),fullPage:true});
 console.log('A team: five templates and six identities verified');
}finally{await owner.close();await browser.close();}
