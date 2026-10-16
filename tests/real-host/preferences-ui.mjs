import {chromium} from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
const root='.cache/real-host/20261012-closure',env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const actors=JSON.parse(fs.readFileSync(path.join(root,'actors/actors.json'),'utf8')),team=JSON.parse(fs.readFileSync(path.join(root,'A-team.json'),'utf8'));
const browser=await chromium.launch();
const records=[];
try{
 for(const [actorID,role,prompt] of [
  ['U1','backend','RH_BACKEND_PRIVATE：先核对 API 与数据一致性，以可复现证据说明边界。此偏好属于后端职位。'],
  ['U1','ops','RH_OPS_PRIVATE：先核对隔离构建与恢复记录，不能用后端实现意见代替运维核对。'],
  ['U3','qa','RH_QA_ALPHA_PRIVATE：优先接口和异常路径核对，独立保存测试甲的依据。'],
  ['U4','qa','RH_QA_BETA_PRIVATE：优先桌面交互、边界和性能对比，独立保存测试乙的依据。'],
 ]){
  const context=await browser.newContext({baseURL:`http://127.0.0.1:${env.port}`,storageState:actors[actorID].statePath,viewport:{width:1440,height:1000}});
  try{
   const page=await context.newPage();await page.goto('/?project='+team.projectID+'&settings=team');
   await page.getByTestId('position-preferences-'+team.positions[role]).click();
   await page.getByTestId('next-personal-prompt').fill(prompt);const saved=page.waitForResponse(r=>r.request().method()==='PUT'&&r.url().includes('preferences'));
   await page.getByTestId('next-save-preference').click();const response=await saved;if(!response.ok())throw new Error('Personal role preference failed');
   records.push({actor:actorID,positionId:team.positions[role],savedVia:'preferences_ui'});console.log(actorID+' saved its independent '+role+' preference');
  }finally{await context.close();}
 }
 fs.writeFileSync(path.join(root,'A-preferences.json'),JSON.stringify(records,null,2)+'\n');
}finally{await browser.close();}
