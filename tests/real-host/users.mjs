// Actual registration UI for the six scene actors. No SQL or business seeds.
import {chromium} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
const directory='.cache/real-host/20261012-closure/actors';
fs.mkdirSync(directory,{recursive:true});
const registryPath=path.join(directory,'actors.json');
const registry=fs.existsSync(registryPath)?JSON.parse(fs.readFileSync(registryPath,'utf8')):{};
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const baseURL=`http://127.0.0.1:${env.port}`;
const browser=await chromium.launch();
try{
 for(const [id,name] of [['U0','联测负责人'],['U1','后端与运维'],['U2','前端与设计制作'],['U3','测试甲与品牌审阅'],['U4','测试乙与数据分析'],['U5','测试甲接岗人']]){
  if(registry[id])continue;
  const context=await browser.newContext({baseURL,viewport:{width:1440,height:1000}});
  try{
   const page=await context.newPage();await page.goto('/register');
   const email=id.toLowerCase()+'-'+randomUUID().slice(0,8)+'@realhost.judex.test',password=randomUUID()+'aA9!';
   await page.locator('input[name="displayName"]').fill(name);
   await page.locator('input[name="email"]').fill(email);
   await page.locator('input[name="password"]').fill(password);
   await page.locator('input[name="passwordConfirm"]').fill(password);
   const responsePromise=page.waitForResponse(r=>r.url().endsWith('/api/v1/auth/register')&&r.request().method()==='POST');
   await page.locator('button[type="submit"]').click();const response=await responsePromise;
   if(response.status()!==201)throw new Error('UI registration failed for '+id);
   const result=(await response.json()).data;
   await page.waitForURL(url=>url.pathname!=='/register');
   const statePath=path.join(directory,id+'.json');await context.storageState({path:statePath});
   registry[id]={id,name,email,userId:result.user.id,statePath,createdVia:'registration_ui'};
   fs.writeFileSync(registryPath,JSON.stringify(registry,null,2)+'\n',{mode:0o600});
   await page.screenshot({path:path.join(directory,id+'-registered.png'),fullPage:true});
   console.log(id+' created via registration UI');
  }finally{await context.close();}
 }
}finally{await browser.close();}
