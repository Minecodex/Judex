import {chromium} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
const [id,actorID,reason]=process.argv.slice(2);
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),team=JSON.parse(fs.readFileSync(path.join(root,'A-team.json'),'utf8'));
const browser=await chromium.launch(),context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors[actorID].statePath,viewport:{width:1440,height:1000}}),page=await context.newPage();
try{
 if(!/^[0-9a-f-]{36}$/.test(id??'')||!reason)throw new Error('Observed proposal and concrete reason required');
 await page.goto('/projects/'+team.projectID+'?tab=decisions&view=proposal&item='+id);
 await page.getByTestId('approve-proposal-'+id).waitFor({state:'visible'});
 await page.getByRole('dialog').getByRole('textbox').fill(reason);
 const result=page.waitForResponse(r=>r.url().endsWith('/proposals/'+id+'/decisions')&&r.request().method()==='POST');
 await page.getByRole('dialog').getByRole('button',{name:'拒收并说明',exact:true}).click();const response=await result;
 if(!response.ok())throw new Error('Human rejection failed: '+await response.text());
 fs.writeFileSync(path.join(root,'proposal-rejection-'+id+'.json'),JSON.stringify({actor:actorID,proposalId:id,reason,rejectedVia:'frozen_review_ui',result:(await response.json()).data},null,2)+'\n');
 console.log(actorID+' rejected the actual proposal with its concrete correction reason');
}finally{await context.close();await browser.close();}
