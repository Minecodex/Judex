import {chromium} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import {scenes} from './scene-definitions.mjs';
const scene=process.argv[2]??'A';
if(!['A','B','C'].includes(scene))throw new Error('Approved scene required');
const folder=scene==='A'?'A-MiniIssue':scenes[scene].folder;
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),team=JSON.parse(fs.readFileSync(path.join(root,scene+'-team.json'),'utf8'));
const reviewed=JSON.parse(fs.readFileSync(path.join(root,scene+'-workflows-reviewed.json'),'utf8'));
if(reviewed.projectID!==team.projectID)throw new Error('Published workflows belong to another scene');
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors.U0.statePath,viewport:{width:1440,height:1000}});
const page=await context.newPage(),prefix='/api/v1/projects/'+team.projectID;
const read=async suffix=>(await(await context.request.get(prefix+suffix)).json()).data;
const choose=async(id,value)=>{await page.getByTestId(id).click();await page.locator('[data-option-value='+JSON.stringify(value)+']').click();await page.getByRole('listbox',{includeHidden:true}).waitFor({state:'detached'});};
const flows=[];
try{
 for(const item of reviewed.workflows){const version=(await read('/workflows/'+item.id+'/versions')).items.find(v=>v.id===item.publishedVersionId);if(!version||version.state!=='published')throw new Error('Reviewed published version required');flows.push({id:item.id,versionId:version.id,publishedVersionId:version.id,state:version.state,body:version.body});}
 for(const [role,id]of Object.entries(team.positions)){
  const bindings=flows.flatMap(flow=>flow.body.nodes.filter(n=>n.allowedPositionIds.includes(id)).map(n=>({workflowId:flow.id,nodeId:n.id})));
  await page.goto('/?project='+team.projectID+'&settings=team');await page.getByTestId('edit-position-'+id).click();
  while(await page.locator('[data-testid^="position-binding-remove-"]').count())await page.locator('[data-testid^="position-binding-remove-"]').first().click();
  for(let index=0;index<bindings.length;index++){
   await page.getByTestId('position-binding-add').click();await choose(index?'position-flow-'+index:'position-flow',bindings[index].workflowId);await choose(index?'position-node-'+index:'position-node',bindings[index].nodeId);
  }
  await page.getByTestId('save-position').click();await page.getByRole('dialog').waitFor({state:'detached'});
  const saved=(await read('/positions')).items.find(p=>p.id===id);if(saved.nodeBindings.length!==bindings.length)throw new Error('Multi-flow bindings not retained');
  console.log(role+' bound to '+bindings.length+' real published nodes through UI');
 }
 fs.writeFileSync(path.join(env.localRoot,folder,'workflow-context.json'),JSON.stringify({projectId:team.projectID,workflows:flows},null,2)+'\n');
 await page.screenshot({path:path.join(root,scene+'-multi-workflow-bindings.png'),fullPage:true});
}finally{await context.close();await browser.close();}
