// Human test owner reviews the actual GLM draft, edits it and publishes via UI.
import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {scenes} from './scene-definitions.mjs';
const scene=process.argv[2]??'A';
if(!['A','B','C'].includes(scene))throw new Error('Approved scene required');
const count=scene==='A'?4:scenes[scene].workflowCount;
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),team=JSON.parse(fs.readFileSync(path.join(root,scene+'-team.json'),'utf8'));
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors.U0.statePath,viewport:{width:1440,height:1000}});
const page=await context.newPage(),prefix='/api/v1/projects/'+team.projectID;
const read=async suffix=>(await(await context.request.get(prefix+suffix)).json()).data;
const file=path.join(root,scene+'-workflows-reviewed.json'),evidence=fs.existsSync(file)?JSON.parse(fs.readFileSync(file,'utf8')):{projectID:team.projectID,workflows:[]};
if(evidence.projectID!==team.projectID)throw new Error('Workflow evidence belongs to another project');
try{
 const heads=(await read('/workflows')).items;
 const names=new RegExp(`^${scene}-W[1-${count}] `);
 const selected=heads.filter(f=>names.test(f.name)).sort((a,b)=>a.name.localeCompare(b.name));if(selected.length!==count)throw new Error('Exact real business drafts required');
 for(const head of selected){
  if(evidence.workflows.some(f=>f.id===head.id&&f.published))continue;
  const draft=(await read('/workflows/'+head.id+'/versions')).items.find(v=>v.state==='draft');if(!draft)throw new Error('Expected model draft');
  const body=draft.body,min=/^A-W1/.test(head.name)?9:5;
  if(body.nodes.length<min||body.nodes.some(n=>n.allowedPositionIds.some(id=>!Object.values(team.positions).includes(id))))throw new Error('Draft roles/structure differ from reviewed scene');
  if(body.nodes.some(n=>(n.delegationUserIds??[]).length))throw new Error('No human proxy-approval grants were requested for this scene');
  if(/^A-W1/.test(head.name)&&body.nodes.filter(n=>n.allowedPositionIds.includes(team.positions.qa)).length<2)throw new Error('Two independent QA duties missing');
  const review={id:head.id,originalName:head.name,modelDraftVersionId:draft.id,modelDraftHash:draft.draftHash,originalBody:body,published:false,reviewer:'U0'};
  evidence.workflows=evidence.workflows.filter(f=>f.id!==head.id);evidence.workflows.push(review);fs.writeFileSync(file,JSON.stringify(evidence,null,2)+'\n');
  await page.goto('/?project='+team.projectID+'&settings=flows');await page.getByRole('button',{name:head.name,exact:true}).click();
  await page.getByTestId('workflow-draft-name').fill(head.name.replace(/（发布前草稿）/g,''));
  const extra=scene==='A'?'测试甲与测试乙的身份、成果和接收分别记录。':'制作、独立核对与最终验收分别记录，固定原件版本及未完成条件必须保留。';
  await page.getByTestId('workflow-draft-instructions').fill(body.instructions+'\nU0 审阅补充：正式计划任务仍须提交固定提案并由列明的负责人确认；'+extra+'工作流发布不代表任一任务已经开始、完成或验收。');
  const saved=page.waitForResponse(r=>r.request().method()==='PUT'&&r.url().endsWith('/workflows/'+head.id+'/draft'));
  await page.getByTestId('save-workflow-draft').click();const saveResponse=await saved;if(!saveResponse.ok())throw new Error('Reviewed draft edit failed');
  await expect(page.getByTestId('publish-workflow-draft')).toBeEnabled();
  const publish=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/workflows/'+head.id+'/publish'));
  await page.getByTestId('publish-workflow-draft').click();const response=await publish;if(!response.ok())throw new Error('Human workflow publication failed');
  const published=(await response.json()).data;await page.getByTestId('workflow-draft-editor').waitFor({state:'detached'});
  Object.assign(review,{published:true,publishedVersionId:published.id,editedVia:'draft_editor_ui',publishedVia:'owner_workflow_ui'});
  fs.writeFileSync(file,JSON.stringify(evidence,null,2)+'\n');console.log('U0 edited and published '+head.name);
 }
 await page.screenshot({path:path.join(root,scene+'-workflows-published.png'),fullPage:true});
}finally{await context.close();await browser.close();}
