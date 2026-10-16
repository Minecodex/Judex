import {chromium,expect} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
const [code,actorID,workPath='.cache/real-host/20261012-closure/A-P1-approved.json']=process.argv.slice(2);
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),work=JSON.parse(fs.readFileSync(workPath,'utf8'));
const task=work.tasks.find(t=>t.title.startsWith(code+' '));if(!task||!actors[actorID])throw new Error('Actual formal task and actor required');
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors[actorID].statePath,viewport:{width:1440,height:1000}}),page=await context.newPage();
try{
 const prefix='/api/v1/projects/'+work.projectId;
 const current=(await(await context.request.get(prefix+'/tasks/'+task.id)).json()).data;
 if(current.status==='working'){console.log(code+' already working');process.exitCode=0;}
 else{
  if(current.status!=='ready')throw new Error('Task is not ready to start');
  await page.goto('/projects/'+work.projectId+'/tasks/'+task.id);
  await page.getByRole('button',{name:'我开始处理',exact:true}).click();
  const confirm=page.getByTestId('confirm-start-task');
  if(await confirm.isVisible()){await confirm.click();await confirm.waitFor({state:'detached'});}
  await expect.poll(async()=>(await(await context.request.get(prefix+'/tasks/'+task.id)).json()).data?.status,{timeout:15000}).toBe('working');
  console.log(actorID+' started '+code+' through its actual task/responsibility UI');
 }
}finally{await context.close();await browser.close();}
