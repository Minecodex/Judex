import {test,expect,type Page} from '@playwright/test';
import {randomUUID,createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import {command,setupCooperation} from './cooperation-business-helpers';
import {uploadMaterial,stableMaterialDialog,chooseMaterialContext} from './materials-business-helpers';
test.setTimeout(240000);
const fixtures=path.resolve('tests/fixtures/materials');
function fixtureSQL(sql:string){
  const namespace=path.basename(process.env.JUDEX_E2E_ARTIFACT??'');
  expect(namespace).toMatch(/^judex-ui-[a-f0-9]{8}$/);
  const ns=JSON.parse(execFileSync('kubectl',['get','namespace',namespace,'-o','json'],{encoding:'utf8',windowsHide:true}));
  expect(ns.metadata.labels['judex.dev/test-run']).toBe(namespace);
  return execFileSync('kubectl',['-n',namespace,'exec','statefulset/ui-postgresql','--','psql','-U','judex','-d','judex','-v','ON_ERROR_STOP=1','-At','-c',sql],{encoding:'utf8',windowsHide:true});
}
async function chooseContext(page:Page,value:string){await page.getByTestId('collaboration-compose-task').click();await page.locator('[data-option-value="'+value+'"]').click();}
async function addExisting(page:Page){
  const choose=page.getByRole('button',{name:'选择共享资料',exact:true});
  if(!await choose.isVisible())await page.getByRole('button',{name:'添加资料',exact:true}).click();
  await choose.click();await stableMaterialDialog(page);
  await page.getByRole('dialog').locator('.judex-material-picker-card>strong').filter({hasText:/^acceptance\.md$/}).click();await page.getByRole('button',{name:'添加到消息',exact:true}).click();
}
async function prepare(page:Page,root:string,id:string){await command(page,'post',root+'/material-versions/'+id+'/preview');await expect.poll(async()=>(await command(page,'get',root+'/material-versions/'+id+'/preview')).status,{timeout:120000}).not.toBe('pending');return command(page,'get',root+'/material-versions/'+id+'/preview');}

test('MAT10 explicit task, plan and whole discussion file contexts survive tabs and panel changes',async({browser},info)=>{
  const f=await setupCooperation(browser),{a,ca,cb,root,task,plan,identity,flow}=f;
  try{
    const otherPlan=await command(a,'post',root+'/plans',{title:'第二用途计划',ownerIdentityId:identity.id});
    const otherTask=await command(a,'post',root+'/tasks',{title:'第二用途任务',planId:otherPlan.id,participantIdentityIds:[identity.id],reviewerIdentityId:identity.id});
    const topic=await command(a,'post',root+'/topics',{title:'多对象资料上下文',links:[{objectType:'task',objectId:task.id},{objectType:'task',objectId:otherTask.id},{objectType:'plan',objectId:plan.id},{objectType:'plan',objectId:otherPlan.id}]}),other=await command(a,'post',root+'/topics',{title:'另一会话',links:[{objectType:'task',objectId:task.id}]});
    const file=await uploadMaterial(a,root,'acceptance.md'),switchBack=async()=>{await a.getByTestId('tab-topic:'+other.id).click();await a.getByTestId('tab-topic:'+topic.id).click();};
    for(let index=0;index<20;index++)await command(a,'post',root+'/submissions',{clientSubmissionId:randomUUID(),purpose:'message',topicId:topic.id,text:'用于非零阅读位置回归的历史讨论 '+index});
    await a.goto(root+'/tasks/'+task.id+'/chat/'+topic.id+'?view=resources');await a.getByTestId('scoped-topic-'+other.id).click();await a.getByTestId('tab-topic:'+topic.id).click();
    for(const [context,expectedTask,expectedPlan,label] of [[otherTask.id,otherTask.id,otherPlan.id,'task'],['',null,null,'topic']] as const){
      await addExisting(a);await chooseContext(a,context);await a.getByTestId('work-discussion-input').fill('EXPLICIT_MATERIAL_'+label);
      const thread=a.getByTestId('chat-thread');await thread.evaluate(e=>{e.scrollTop=100;});await expect.poll(()=>thread.evaluate(e=>e.scrollTop)).toBeGreaterThan(0);const position=await thread.evaluate(e=>e.scrollTop);await switchBack();
      await expect(a.getByTestId('collaboration-compose-task')).toHaveAttribute('data-value',context);await expect(a.getByTestId('workspace-tab-resources')).toHaveAttribute('aria-selected','true');await expect(a.getByTestId('work-discussion-input')).toHaveValue('EXPLICIT_MATERIAL_'+label);await expect(a.locator('.judex-work-upload')).toContainText('acceptance.md');
      await expect.poll(()=>thread.evaluate(e=>e.scrollTop)).toBeCloseTo(position,0);
      await a.getByTestId('chat-pane-toggle').click();await a.getByTestId('chat-pane-toggle').click();await expect(a.getByTestId('collaboration-compose-task')).toHaveAttribute('data-value',context);
      await a.getByTestId('send-work-message').click();await expect.poll(async()=>(await command(a,'get',root+'/material-versions/'+file.id+'/usages')).items.some((v:any)=>v.description==='EXPLICIT_MATERIAL_'+label&&v.taskId===expectedTask&&v.planId===expectedPlan)).toBe(true);
    }
    await a.getByTestId('work-discussion-input').fill('EXPLICIT_MATERIAL_plan');
    await a.locator('.judex-material-library-panel .judex-material-card').getByRole('button',{name:'详情',exact:true}).click();await stableMaterialDialog(a);await a.getByRole('dialog').getByRole('button',{name:'发送到讨论',exact:true}).click();await stableMaterialDialog(a);
    await chooseMaterialContext(a,1,'plan:'+otherPlan.id);await a.getByRole('dialog').getByRole('button',{name:'添加到消息',exact:true}).click();await switchBack();
    await expect(a.getByTestId('collaboration-compose-task')).toHaveAttribute('data-value','plan:'+otherPlan.id);await expect(a.getByTestId('work-discussion-input')).toHaveValue('EXPLICIT_MATERIAL_plan');
    await a.getByTestId('workspace-maximize').click();await a.getByTestId('workspace-maximize').click();
    await expect(a.getByTestId('collaboration-compose-task')).toHaveAttribute('data-value','plan:'+otherPlan.id);await a.getByTestId('send-work-message').click();
    await expect.poll(async()=>(await command(a,'get',root+'/material-versions/'+file.id+'/usages')).items.some((v:any)=>v.description==='EXPLICIT_MATERIAL_plan'&&v.planId===otherPlan.id&&v.taskId===null)).toBe(true);
    expect((await command(a,'get',root+'/tasks/'+task.id)).planId).toBe(plan.id);expect((await command(a,'get',root+'/tasks/'+task.id)).status).toBe('ready');expect((await command(a,'get',root+'/tasks/'+otherTask.id)).planId).toBe(otherPlan.id);
    await a.screenshot({path:info.outputPath('preserved-material-work-context.png')});
  }finally{await ca.close();await cb.close();}
});

test('MAT11 damaged files fail, legacy caches revalidate and transient read errors recover coherently',async({browser},info)=>{
  const f=await setupCooperation(browser),{a,ca,cb,root,project}=f;
  try{
    const damaged=[['truncated.pdf',Buffer.from('%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntruncated')],['truncated.png',fs.readFileSync(path.join(fixtures,'acceptance.png')).subarray(0,33)]] as const;
    for(const [name,raw] of damaged){const file=await uploadMaterial(a,root,name,raw);expect((await prepare(a,root,file.id)).status).toBe('failed');const original=await a.request.get('/api/v1'+root+'/material-versions/'+file.id+'/content');expect((await original.body()).equals(raw)).toBe(true);
      await a.goto(root+'?tab=materials');const card=a.locator('.judex-material-card').filter({hasText:name});await expect(card.locator('.judex-material-status')).toHaveAttribute('data-preview-status','failed');await card.locator('.judex-material-cover').click();await stableMaterialDialog(a);await expect(a.getByRole('dialog').locator('.judex-material-status')).toContainText('内容预览失败');await a.getByRole('dialog').getByRole('button',{name:'重试',exact:true}).click();await expect.poll(async()=>(await command(a,'get',root+'/material-versions/'+file.id+'/preview')).status).toBe('failed');await a.getByRole('button',{name:'关闭',exact:true}).last().click();
    }
    const pdf=await uploadMaterial(a,root,'acceptance.pdf');expect((await prepare(a,root,pdf.id)).status).toBe('ready');expect(pdf.id).toMatch(/^[0-9a-f-]{36}$/);
    fixtureSQL(`UPDATE material_previews SET validation_revision=0 WHERE version_id='${pdf.id}'`);
    expect((await command(a,'get',root+'/material-versions/'+pdf.id+'/preview')).status).toBe('not_requested');expect((await command(a,'get',root+'/materials')).items.find((v:any)=>v.versionId===pdf.id).previewStatus).toBe('not_requested');expect((await prepare(a,root,pdf.id)).status).toBe('ready');
    const endpoint='**/api/v1'+root+'/material-versions/'+pdf.id+'/preview/content';await a.route(endpoint,r=>r.abort('failed'));await a.goto(root+'?tab=materials');const card=a.locator('.judex-material-card').filter({hasText:'acceptance.pdf'});
    await expect(card.locator('.judex-material-status')).toHaveAttribute('data-preview-status','read_failed');await card.locator('.judex-material-cover').click();await stableMaterialDialog(a);await expect(a.getByRole('dialog').locator('.judex-material-status')).toContainText('读取文件失败');
    expect((await command(a,'get',root+'/material-versions/'+pdf.id+'/preview')).status).toBe('ready');await a.unroute(endpoint);await a.getByRole('dialog').getByRole('button',{name:'重试',exact:true}).click();await expect(a.getByRole('dialog').locator('.judex-material-reader-toolbar')).toContainText('1 / 2');await expect(a.getByRole('dialog').locator('.judex-material-status')).toHaveAttribute('data-preview-status','ready');
    const original=await a.request.get('/api/v1'+root+'/material-versions/'+pdf.id+'/content');expect(createHash('sha256').update(await original.body()).digest('hex')).toBe(createHash('sha256').update(fs.readFileSync(path.join(fixtures,'acceptance.pdf'))).digest('hex'));
    await a.screenshot({path:info.outputPath('validated-preview-read-recovery.png')});
  }finally{await ca.close();await cb.close();}
});

test('MAT12 first nonempty formal purpose skips blank history and discussion while preserving later uses',async({browser},info)=>{
  const f=await setupCooperation(browser),{a,ca,cb,root,project,task}=f;
  try{
    const file=await uploadMaterial(a,root,'historic-purpose.txt',Buffer.from('immutable original'),''),actor=(await command(a,'get','/auth/session')).user.id;
    for(const id of [file.id,project.id,task.id,actor])expect(id).toMatch(/^[0-9a-f-]{36}$/);
    fixtureSQL(`INSERT INTO material_links(project_id,id,version_id,object_type,object_id,purpose,confirmed_by,created_at) VALUES('${project.id}','${randomUUID()}','${file.id}','task','${task.id}','   ','${actor}',now()-interval '1 day')`);
    const topic=await command(a,'post',root+'/topics',{title:'用途回退讨论'});await command(a,'post',root+'/submissions',{clientSubmissionId:randomUUID(),purpose:'message',text:'Earlier discussion text is not the formal upload purpose',topicId:topic.id,materialVersionIds:[file.id]});
    for(const text of ['第一份正式材料用途说明','后续独立使用说明'])await command(a,'post',root+'/submissions',{clientSubmissionId:randomUUID(),purpose:'material',text,taskId:task.id,materialVersionIds:[file.id]});
    expect((await command(a,'get',root+'/material-versions/'+file.id)).library.purpose).toBe('第一份正式材料用途说明');const uses=(await command(a,'get',root+'/material-versions/'+file.id+'/usages')).items;expect(uses).toHaveLength(4);expect(uses.some((v:any)=>v.description==='后续独立使用说明')).toBe(true);
    await a.goto(root+'?tab=materials');await a.locator('.judex-material-card').getByRole('button',{name:'详情',exact:true}).click();await stableMaterialDialog(a);await expect(a.getByRole('dialog').locator('.judex-material-purpose')).toContainText('第一份正式材料用途说明');await expect(a.getByRole('dialog').locator('.judex-material-timeline')).toContainText('后续独立使用说明');await a.screenshot({path:info.outputPath('first-formal-purpose.png')});
  }finally{await ca.close();await cb.close();}
});
