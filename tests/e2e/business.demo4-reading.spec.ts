import {test,expect} from '@playwright/test';
import {setupCooperation,command} from './cooperation-business-helpers';

test('D4-R7 real paginated records restore originals, tabs, reload and independent fullscreen state',async({browser},info)=>{
 test.setTimeout(120000);
 const f=await setupCooperation(browser),{a,root,task,identity,plan}=f;
 try{
  await command(a,'post',root+'/tasks/'+task.id+'/start',{expectedVersion:task.version,identityId:identity.id});
  for(let i=0;i<23;i++){
   const current=await command(a,'get',root+'/tasks/'+task.id);
   await command(a,'post',root+'/tasks/'+task.id+'/reports',{kind:'progress',text:('READING_'+i+' 真实进展、职责材料与核对依据。').repeat(16),identityId:identity.id,expectedTaskVersion:current.version});
  }
  const history=await command(a,'get',root+'/tasks/'+task.id+'/activity?limit=100');
  expect(history.items.length).toBeGreaterThan(20);expect(history.nextCursor).toBeFalsy();
  const originalId=history.items[0].id;
  await a.goto(`${root}/plans/${plan.id}/chat/${plan.mainTopicId}`);
  await a.getByTestId('work-discussion-input').fill('保留当前讨论的未发送内容');
  await a.locator('.judex-co-context-task').filter({hasText:'登录功能'}).click();
  await a.getByTestId('workspace-add-tool').click();await a.getByRole('menuitem',{name:'任务记录',exact:true}).click();
  const records=a.getByTestId('task-section-records'),scroll=a.locator('.judex-task-scroll');
  await records.getByRole('button',{name:'加载更早记录',exact:true}).click();
  await expect(records.locator('[data-activity]')).toHaveCount(history.items.length);
  const original=records.locator('[data-activity="'+originalId+'"]').getByRole('button',{name:'展开原始报告',exact:true});
  await original.click();await expect(original).toHaveAttribute('aria-expanded','true');
  await original.evaluate(el=>Promise.all(el.closest('.judex-ui-disclosure')!.getAnimations({subtree:true}).map(a=>a.finished.catch(()=>{}))));
  await scroll.evaluate(el=>{el.scrollTop=900;el.dispatchEvent(new Event('scroll'));});
  await a.screenshot({path:info.outputPath('history-before-switch.png'),fullPage:true});
  await a.getByTestId('workspace-tab-task:'+task.id+':overview').click();
  await a.getByTestId('workspace-tab-task:'+task.id+':records').click();
  await expect(records.locator('[data-activity]')).toHaveCount(history.items.length);
  await expect(original).toHaveAttribute('aria-expanded','true');
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
  await expect(a.getByTestId('work-discussion-input')).toHaveValue('保留当前讨论的未发送内容');
  await a.reload();
  await expect(records.locator('[data-activity]')).toHaveCount(history.items.length);
  await expect(original).toHaveAttribute('aria-expanded','true');
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
  await a.goto(`${root}/plans/${plan.id}/route?task=${task.id}&section=records`);
  await expect(records.locator('[data-activity]')).toHaveCount(history.items.length);
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
  await a.getByRole('button',{name:'全屏查看',exact:true}).click();
  const viewer=a.getByTestId('route-fullscreen');await expect(viewer).toBeVisible();
  await expect(records.locator('[data-activity]')).toHaveCount(history.items.length);
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
  await a.getByTestId('task-record-source').click();await a.locator('[data-option-value="cli"]').click();
  await expect(records.locator('[data-activity]')).toHaveCount(0);
  await viewer.getByTestId('exit-route-fullscreen').click();
  await expect(a.getByTestId('task-record-source')).toHaveAttribute('data-value','all');
  await expect(records.locator('[data-activity]')).toHaveCount(history.items.length);
  await expect(original).toHaveAttribute('aria-expanded','true');
  await expect.poll(()=>scroll.evaluate(el=>el.scrollTop)).toBe(900);
  await a.screenshot({path:info.outputPath('history-after-return.png'),fullPage:true});
  const latest=await command(a,'get',root+'/tasks/'+task.id+'/activity?limit=100');
  expect(latest.items.map((v:any)=>v.id)).toEqual(history.items.map((v:any)=>v.id));
  await info.attach('reading-state',{body:JSON.stringify({taskId:task.id,rows:history.items.length,originalId,position:900,pagesRestored:true,centralDraftPreserved:true,fullscreenIsolated:true}),contentType:'application/json'});
 }finally{await f.ca.close();await f.cb.close();}
});
