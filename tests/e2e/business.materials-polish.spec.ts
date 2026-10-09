import {test,expect,type Page} from '@playwright/test';
import fs from 'node:fs';
import {command,setupCooperation} from './cooperation-business-helpers';
import {uploadMaterial,stableMaterialDialog} from './materials-business-helpers';
import {authZh,authEn} from '../../web/src/i18n/auth';
test.setTimeout(240000);
const serverFailure={error:{code:'DEPENDENCY_UNAVAILABLE',message:'Raw server exception must not appear in the interface'}};
async function selectContext(page:Page,value:string){await page.getByTestId('collaboration-compose-task').click();await page.locator('[data-option-value="'+value+'"]').click();await expect(page.locator('.judex-ui-select-popover')).toHaveCount(0);}
function widePDF(){
  const stream=(text:string)=>`BT /F1 28 Tf 40 620 Td (${text}) Tj ET\n`;
  const one=stream('WIDE_PAGE_ONE'),two=stream('WIDE_PAGE_TWO');
  const objects=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>',
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 1200 700] /Resources << /Font << /F1 7 0 R >> >> /Contents 4 0 R >>',`<< /Length ${one.length} >>\nstream\n${one}endstream`,
    '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 1200 700] /Resources << /Font << /F1 7 0 R >> >> /Contents 6 0 R >>',`<< /Length ${two.length} >>\nstream\n${two}endstream`,'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>'];
  let source='%PDF-1.4\n',offsets=[0];for(let i=0;i<objects.length;i++){offsets.push(Buffer.byteLength(source));source+=`${i+1} 0 obj\n${objects[i]}\nendobj\n`;}
  const xref=Buffer.byteLength(source);source+=`xref\n0 8\n0000000000 65535 f \n`+offsets.slice(1).map(offset=>String(offset).padStart(10,'0')+' 00000 n \n').join('');
  return Buffer.from(source+`trailer\n<< /Size 8 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`);
}

test('MAT13 upload registration honors explicit discussion context and retries the same uploaded versions',async({browser},info)=>{
 const f=await setupCooperation(browser),{a,ca,cb,root,task,plan,identity}=f;
 try{
  const secondPlan=await command(a,'post',root+'/plans',{title:'另一个登记计划',ownerIdentityId:identity.id}),secondTask=await command(a,'post',root+'/tasks',{title:'另一个登记任务',planId:secondPlan.id,participantIdentityIds:[identity.id],reviewerIdentityId:identity.id});
  const topic=await command(a,'post',root+'/topics',{title:'登记上下文',links:[{objectType:'plan',objectId:plan.id},{objectType:'plan',objectId:secondPlan.id},{objectType:'task',objectId:task.id},{objectType:'task',objectId:secondTask.id}]});
  await a.goto(root+'/tasks/'+task.id+'/chat/'+topic.id+'?view=resources');const topics=(await command(a,'get',root+'/topics')).totalCount;
  for(const [context,wantedPlan,wantedTask,label]of [['','','','project'],['plan:'+secondPlan.id,secondPlan.id,'','plan'],[secondTask.id,secondPlan.id,secondTask.id,'task']] as const){
   await selectContext(a,context);await a.locator('.judex-material-library-panel').getByRole('button',{name:'上传资料',exact:true}).click();await stableMaterialDialog(a);
   const dialog=a.getByRole('dialog');await expect(dialog.locator('.judex-ui-select [data-value]').nth(0)).toHaveAttribute('data-value',wantedPlan);await expect(dialog.locator('.judex-ui-select [data-value]').nth(1)).toHaveAttribute('data-value',wantedTask);
   await dialog.locator('input[type=file]').setInputFiles({name:label+'.txt',mimeType:'text/plain',buffer:Buffer.from('CONTEXT_REGISTRATION_'+label)});await dialog.getByRole('textbox',{name:'材料用途',exact:true}).fill('用途 '+label);
   const submitted=a.waitForResponse(r=>r.url().endsWith(root+'/submissions')&&r.request().method()==='POST');await dialog.getByRole('button',{name:'登记资料',exact:true}).click();const response=await submitted;expect(response.ok()).toBe(true);
   const payload=response.request().postDataJSON(),version=payload.materialVersionIds[0];expect(payload.taskId??'').toBe(wantedTask);expect(payload.planId??'').toBe(wantedTask?'':wantedPlan);
   await expect(a.getByRole('dialog')).toHaveCount(0);await expect(a.getByTestId('workspace-tab-resources')).toHaveAttribute('aria-selected','true');
   const use=(await command(a,'get',root+'/material-versions/'+version+'/usages')).items.find((v:any)=>v.kind==='material');expect(use.taskId??'').toBe(wantedTask);expect(use.planId??'').toBe(wantedPlan);
  }
  await selectContext(a,'');await a.locator('.judex-material-library-panel').getByRole('button',{name:'上传资料',exact:true}).click();await stableMaterialDialog(a);
  const attempts:any[]=[];let first=true;await a.route('**/api/v1'+root+'/submissions',async route=>{attempts.push({body:route.request().postDataJSON(),key:route.request().headers()['idempotency-key']});if(first){first=false;await route.fulfill({status:503,json:serverFailure});}else await route.continue();});
  const dialog=a.getByRole('dialog');await dialog.locator('input[type=file]').setInputFiles({name:'registration-retry.txt',mimeType:'text/plain',buffer:Buffer.from('RETRY_EXISTING_ORIGINAL')});await dialog.getByRole('textbox',{name:'材料用途',exact:true}).fill('登记重试保持原件');await dialog.getByRole('button',{name:'登记资料',exact:true}).click();
  await expect(dialog.getByRole('alert')).toHaveText(authZh.errServer);await expect(dialog).toContainText('重试会复用已上传版本');await dialog.getByRole('button',{name:'登记资料',exact:true}).click();await expect(a.getByRole('dialog')).toHaveCount(0);
  expect(attempts).toHaveLength(2);expect(attempts[1]).toEqual(attempts[0]);expect((await command(a,'get',root+'/materials')).totalCount).toBe(4);expect((await command(a,'get',root+'/topics')).totalCount).toBe(topics);expect((await command(a,'get',root+'/tasks/'+task.id)).planId).toBe(plan.id);
  await a.screenshot({path:info.outputPath('registered-explicit-context.png')});
 }finally{await ca.close();await cb.close();}
});

test('MAT14 PDF has visible loading, reflows on desktop resize and preserves page and zoom',async({browser},info)=>{
 const f=await setupCooperation(browser),{a,ca,cb,root}=f;const errors:string[]=[];a.on('pageerror',error=>errors.push(error.message));
 try{
  const file=await uploadMaterial(a,root,'wide.pdf',widePDF());await command(a,'post',root+'/material-versions/'+file.id+'/preview');await expect.poll(async()=>(await command(a,'get',root+'/material-versions/'+file.id+'/preview')).status,{timeout:120000}).toBe('ready');
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});await a.route('**/api/v1'+root+'/material-versions/'+file.id+'/preview/content',async route=>{await gate;await route.continue();});
  await a.goto(root+'?tab=materials');await a.locator('.judex-material-cover').click();await stableMaterialDialog(a);const reader=a.getByRole('dialog').locator('.judex-material-pdf');await expect(reader).toHaveAttribute('data-render-state','pending');await expect(reader.getByRole('status')).toContainText('正在读取文件');release();
  await expect(reader).toHaveAttribute('data-render-state','ready');await a.getByRole('dialog').getByRole('button',{name:'下一页',exact:true}).click();await expect(reader).toHaveAttribute('data-render-state','ready');await expect(reader.locator('.judex-material-reader-toolbar')).toContainText('2 / 2');
  await a.getByRole('dialog').getByRole('button',{name:'放大',exact:true}).click();await expect(reader).toHaveAttribute('data-render-state','ready');const before=await reader.locator('canvas').evaluate((canvas:HTMLCanvasElement)=>canvas.width);
  await a.setViewportSize({width:1120,height:1000});await expect.poll(()=>reader.locator('canvas').evaluate((canvas:HTMLCanvasElement)=>canvas.width)).toBeLessThan(before);await expect(reader).toHaveAttribute('data-render-state','ready');await expect(reader.locator('.judex-material-reader-toolbar')).toContainText('2 / 2');await expect(reader.locator('.judex-material-reader-toolbar')).toContainText('125%');
  await a.setViewportSize({width:1920,height:1000});await expect.poll(()=>reader.locator('canvas').evaluate((canvas:HTMLCanvasElement)=>canvas.width)).toBeGreaterThan(before-1);await expect(reader).toHaveAttribute('data-render-state','ready');expect(errors).toEqual([]);await a.screenshot({path:info.outputPath('resized-real-pdf.png')});
 }finally{await ca.close();await cb.close();}
});

test('MAT15 isolated HTML preview obtains a new session after a transient preparation failure',async({browser},info)=>{
 const f=await setupCooperation(browser),{a,ca,cb,root}=f;
 try{
  const file=await uploadMaterial(a,root,'preview.zip',fs.readFileSync('tests/fixtures/preview.zip'),'用于隔离预览重试',{kind:'html_bundle',entrypoint:'index.html'});
  const endpoint='**/api/v1'+root+'/materials/'+file.materialId+'/versions/'+file.id+'/preview-session';await a.route(endpoint,route=>route.abort('failed'));await a.goto(root+'?tab=materials');await a.locator('.judex-material-cover').click();await stableMaterialDialog(a);await expect(a.getByRole('dialog').getByRole('alert')).toContainText('读取文件失败');
  await a.unroute(endpoint);await a.getByRole('dialog').getByRole('button',{name:'重试',exact:true}).click();await expect(a.frameLocator('.judex-material-html').getByRole('heading',{name:'隔离 HTML 预览'})).toHaveAttribute('data-executed','yes');
  const url=await a.locator('.judex-material-html').getAttribute('src');expect(new URL(url!).origin).not.toBe(new URL(process.env.JUDEX_E2E_BASE_URL!).origin);expect((await a.context().cookies(url!)).some(cookie=>cookie.name.includes('session'))).toBe(false);await a.screenshot({path:info.outputPath('retried-isolated-html.png')});
 }finally{await ca.close();await cb.close();}
});

test('MAT16 delete uses localized failures, guards pending dismissal and retries without losing history',async({browser},info)=>{
 const f=await setupCooperation(browser),{a,ca,cb,root}=f;
 try{for(const locale of ['zh-CN','en']){
  const file=await uploadMaterial(a,root,'localized-delete-'+locale+'.txt',Buffer.from('ORIGINAL_DELETE_EVIDENCE'));await a.goto(root+'?tab=materials');await a.evaluate(locale=>localStorage.setItem('argus.locale',locale),locale);await a.reload();const zh=locale==='zh-CN';
  await a.locator('.judex-material-card').getByRole('button',{name:zh?'删除':'Delete',exact:true}).click();await stableMaterialDialog(a);const endpoint='**/api/v1'+root+'/materials/'+file.materialId;let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});await a.route(endpoint,async route=>{await gate;await route.fulfill({status:503,json:serverFailure});});
  const confirm=a.getByRole('dialog').getByRole('button',{name:zh?'确认删除':'Delete file',exact:true});await confirm.click();await expect(confirm).toBeDisabled();await a.keyboard.press('Escape');await expect(a.getByRole('dialog')).toBeVisible();release();await expect(a.getByRole('dialog').getByRole('alert')).toHaveText(zh?authZh.errServer:authEn.errServer);await expect(confirm).toBeEnabled();
  await a.unroute(endpoint);await confirm.click();await expect(a.getByRole('dialog')).toHaveCount(0);await expect(a.locator('.judex-material-card')).toHaveCount(0);expect((await a.request.get('/api/v1'+root+'/material-versions/'+file.id+'/content')).ok()).toBe(true);
 }await a.screenshot({path:info.outputPath('localized-delete-completed.png')});}finally{await ca.close();await cb.close();}
});
