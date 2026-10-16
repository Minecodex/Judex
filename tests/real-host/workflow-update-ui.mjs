// Publish reviewed role corrections through the real owner editor, preserving
// all previous published versions and every unrelated node/rule.
import fs from 'node:fs';
import path from 'node:path';
import {chromium,expect} from '@playwright/test';
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),team=JSON.parse(fs.readFileSync(path.join(root,'A-team.json'),'utf8'));
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors.U0.statePath,viewport:{width:1440,height:1000}}),page=await context.newPage();
const prefix='/api/v1/projects/'+team.projectID;
const read=async suffix=>{const r=await context.request.get(prefix+suffix);if(!r.ok())throw new Error('Workflow read failed '+r.status());return(await r.json()).data;};
const evidencePath=path.join(root,'A-workflows-reviewed.json'),evidence=JSON.parse(fs.readFileSync(evidencePath,'utf8'));
const updates=[];
try{
 const heads=(await read('/workflows')).items.filter(w=>/^A-W[1-4] /.test(w.name));
 if(heads.length!==4)throw new Error('Four actual published scene workflows required');
 for(const head of heads){
  const versions=(await read('/workflows/'+head.id+'/versions')).items,old=versions.find(v=>v.id===head.publishedVersionId);
  if(!old||old.state!=='published'||versions.some(v=>v.state==='draft'))throw new Error('Current published workflow without unreviewed draft required');
  const expected=structuredClone(old.body);
  const oldStatus='状态：发布前草稿，未发布不生效，不创建任何正式计划或任务。';
  if(!expected.instructions.includes(oldStatus)&&expected.instructions.startsWith('当前为已发布协作流程；')&&evidence.workflows.find(w=>w.id===head.id)?.publishedVersionId===old.id){console.log('Previously confirmed revision retained: '+head.name);continue;}
  if(!expected.instructions.includes(oldStatus))throw new Error('Known stale status text required');
  expected.instructions=expected.instructions.replace(oldStatus,'当前为已发布协作流程；发布仅提供职责与建议路径，工作安排、开始、交付与验收仍各自确认。');
  if(head.id==='4b1c2af6-6945-59ce-a04f-43c852313146'){
   const node=expected.nodes.find(n=>n.id==='aw4-restore');if(!node||JSON.stringify(node.allowedPositionIds)!==JSON.stringify([team.positions.ops]))throw new Error('Known restoration role mismatch required');
   node.allowedPositionIds.push(team.positions.backend);
   node.responsibility='后端开发负责在隔离测试环境内按诊断结果恢复服务配置与数据访问；构建运维配合检查启动、回退和运行事实。记录实际步骤、变更、原数据保持与回退路径；仅限本轮明确拥有的隔离实例，部分恢复或未恢复须显性说明，正常部署与无关业务不属于该节点授权范围。';
  }
  await page.goto('/?project='+team.projectID+'&settings=flows&settingsItem='+head.id);await expect(page.getByRole('button',{name:head.name,exact:true})).toHaveAttribute('aria-pressed','true');await page.getByTestId('edit-workflow').click();
  await page.getByTestId('workflow-draft-instructions').fill(expected.instructions);
  if(head.id==='4b1c2af6-6945-59ce-a04f-43c852313146'){
   await page.getByText('3. 恢复',{exact:true}).click();
   await page.getByTestId('workflow-node-responsibility-aw4-restore').fill(expected.nodes.find(n=>n.id==='aw4-restore').responsibility);
   await page.getByTestId('workflow-node-position-aw4-restore-'+team.positions.backend).click();
   await expect(page.getByTestId('workflow-node-positions-aw4-restore').getByRole('checkbox',{name:'后端开发',exact:true})).toBeChecked();
  }
  const saving=page.waitForResponse(r=>r.request().method()==='PUT'&&r.url().endsWith('/workflows/'+head.id+'/draft'));await page.getByTestId('save-workflow-draft').click();const saved=await saving;if(!saved.ok())throw new Error('Owner draft save failed');
  const draft=(await read('/workflows/'+head.id+'/versions')).items.find(v=>v.state==='draft');
  if(!draft||draft.body.instructions!==expected.instructions)throw new Error('Reviewed instruction edit not retained');
  const normalized=body=>JSON.stringify({nodes:body.nodes.map(n=>({id:n.id,name:n.name,responsibility:n.responsibility,positions:[...n.allowedPositionIds].sort(),approval:n.defaultApprovalPolicy,delegation:n.delegationUserIds??[]})),rules:body.hardRules,policies:body.approvalPolicies,edges:body.advisoryEdges});
  if(normalized(draft.body)!==normalized(expected))throw new Error('Unrelated workflow node/rule changed');
  const publishing=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/workflows/'+head.id+'/publish'));await expect(page.getByTestId('publish-workflow-draft')).toBeEnabled();await page.getByTestId('publish-workflow-draft').click();const publishedResponse=await publishing;if(!publishedResponse.ok())throw new Error('Owner publication failed');
  const published=(await publishedResponse.json()).data;
  const history=(await read('/workflows/'+head.id+'/versions')).items;
  if(!history.some(v=>v.id===old.id&&JSON.stringify(v.body)===JSON.stringify(old.body)))throw new Error('Previous published body changed');
  const item=evidence.workflows.find(w=>w.id===head.id);item.originalPublishedVersionId??=item.publishedVersionId;item.publishedVersionId=published.id;
  const change={workflowId:head.id,oldVersion:old.id,newVersion:published.id,actor:'U0',via:'actual owner edit/save/publish UI',roleCorrection:head.id==='4b1c2af6-6945-59ce-a04f-43c852313146',at:new Date().toISOString()};
  (item.publicationRevisions??=[]).push(change);updates.push(change);fs.writeFileSync(evidencePath,JSON.stringify(evidence,null,2)+'\n');console.log('U0 revised and published '+head.name);
 }
 const workflows=[];for(const head of heads){const versions=(await read('/workflows/'+head.id+'/versions')).items;const item=evidence.workflows.find(w=>w.id===head.id),v=versions.find(v=>v.id===item.publishedVersionId);workflows.push({id:head.id,versionId:v.id,publishedVersionId:v.id,body:v.body,state:v.state});}
 fs.writeFileSync(path.join(env.localRoot,'A-MiniIssue/workflow-context.json'),JSON.stringify({projectId:team.projectID,workflows,source:'actual owner publication after role correction'},null,2)+'\n');
 fs.writeFileSync(path.join(root,'workflow-role-update.json'),JSON.stringify(evidence.workflows.flatMap(w=>w.publicationRevisions??[]),null,2)+'\n');await page.screenshot({path:path.join(root,'workflow-role-update.png'),fullPage:true});
}finally{await context.close();await browser.close();}
