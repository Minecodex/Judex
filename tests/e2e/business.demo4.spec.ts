import {test,expect} from '@playwright/test';
import fs from 'node:fs';
import {createHash} from 'node:crypto';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {openTaskAction} from './workspace-helpers';
import {setupCooperation,command,applyCooperation,approveCooperation} from './cooperation-business-helpers';
test('D4-B01 real draft revision, responsibility approval, skip, restore and acceptance',async({browser})=>{
 const f=await setupCooperation(browser),{a,b,root,project,identity,foreign,flow}=f;
 try{
  const plan=await command(a,'post',root+'/plans',{title:'Demo4 真实任务生命周期',goal:'清晰可核对的协作过程',acceptanceCriteria:'原记录和正式决定完整',ownerIdentityId:identity.id,workflowId:flow.id});
  const create=(title:string,requirements:unknown[]=[])=>command(a,'post',root+'/tasks',{planId:plan.id,title,expectedOutput:'提供可核对的成果与依据',acceptanceCriteria:'证据完整',participantIdentityIds:[identity.id,foreign.id],reviewerIdentityId:identity.id,workflowId:flow.id,nodeId:'work',requirements});
  const first=await create('交互评审'),second=await create('功能开发',[{phase:'start',kind:'task_acceptance',targetId:first.id,hard:true,label:'交互评审已验收'}]);
  await a.goto(`/projects/${project.id}/plans/${plan.id}/route`);await expect(a.getByTestId('execution-task-'+first.id)).toBeVisible();await a.getByTestId('task-more-'+first.id).click();await a.getByRole('menuitem',{name:'编辑草稿任务',exact:true}).click();await a.getByTestId('edit-work-description').fill('评审任务卡片、详情抽屉与路线查看，保留核对依据。');await a.getByTestId('save-work-edit').click();await expect(a.getByTestId('execution-task-'+first.id)).toContainText('评审任务卡片');
  const updated=await command(a,'get',root+'/tasks/'+first.id);expect(updated.version).toBe(first.version+1);expect(updated.status).toBe('draft');
  await applyCooperation(a,b,root,[{operation:'activate_object',targetType:'plan',targetId:plan.id,expectedVersion:plan.version},{operation:'activate_object',targetType:'task',targetId:first.id,expectedVersion:updated.version},{operation:'activate_object',targetType:'task',targetId:second.id,expectedVersion:second.version}]);
  const activeFirst=await command(a,'get',root+'/tasks/'+first.id);await command(a,'post',root+`/tasks/${first.id}/start`,{expectedVersion:activeFirst.version,identityId:identity.id});
  await a.reload();await a.getByTestId('task-more-'+first.id).click();await a.getByRole('menuitem',{name:'临时跳过',exact:true}).click();await a.getByTestId('execution-exception-reason').fill('原型已核对，本次先继续实现');await a.getByTestId('exception-waiver-'+second.id).click();await a.getByTestId('confirm-work-mutation').click();await expect(a.getByTestId('execution-task-'+first.id)).toHaveAttribute('data-stage','skipped');await expect(a.getByTestId('execution-task-'+second.id)).toHaveAttribute('data-stage','ready');
  const skipped=await command(a,'get',root+'/tasks/'+first.id);expect(skipped.status).toBe('working');expect(skipped.executionException.reason).toContain('原型');const stats=await command(a,'get',root+'/plans/'+plan.id);expect(stats.taskStats.skipped).toBe(1);expect(stats.taskStats.accepted).toBe(0);expect(stats.taskStats.required).toBe(1);
  const denied=await b.request.get('/api/v1'+root+`/tasks/${first.id}/execution-review?operation=restore`);expect(denied.status()).toBe(403);
  const next=await command(a,'get',root+'/tasks/'+second.id);await command(a,'post',root+`/tasks/${second.id}/start`,{expectedVersion:next.version,identityId:identity.id});
  await a.getByTestId('task-more-'+first.id).click();await a.getByRole('menuitem',{name:'恢复执行',exact:true}).click();await a.getByTestId('execution-exception-reason').fill('需要补充交互核对');await expect(a.getByTestId('confirm-work-mutation')).toBeDisabled();await a.getByTestId('exception-acknowledge').click();await a.getByTestId('confirm-work-mutation').click();await expect(a.locator('.judex-dialog-content')).toHaveCount(0);
  expect((await command(a,'get',root+'/tasks/'+second.id)).status).toBe('working');expect((await command(a,'get',root+'/tasks/'+first.id)).executionException).toBeNull();
  for(const task of [first,second]){
   for(const [page,id] of [[a,identity.id],[b,foreign.id]] as const){const current=await command(page,'get',root+'/tasks/'+task.id);await command(page,'post',root+`/tasks/${task.id}/reports`,{kind:'delivery',text:'已完成本次职责并提供可核对说明',identityId:id,expectedTaskVersion:current.version});expect((await command(page,'get',root+'/tasks/'+task.id)).status).toBe(id===identity.id?'working':'delivered');}
   const review=await command(a,'get',root+`/tasks/${task.id}/acceptance-review`);await command(a,'post',root+`/tasks/${task.id}/acceptances`,{reviewId:review.reviewId,reviewHash:review.reviewHash,expectedVersion:review.targetVersion,decision:'accept'});
  }
  const planReview=await command(a,'get',root+`/plans/${plan.id}/acceptance-review`);await command(a,'post',root+`/plans/${plan.id}/acceptances`,{reviewId:planReview.reviewId,reviewHash:planReview.reviewHash,expectedVersion:planReview.targetVersion,decision:'accept'});expect((await command(a,'get',root+'/plans/'+plan.id)).status).toBe('accepted');
  const history=await command(a,'get',root+`/tasks/${first.id}/activity`);expect(history.items.filter((r:any)=>r.kind==='decision')).toHaveLength(3);expect(history.items.filter((r:any)=>r.kind==='delivery')).toHaveLength(2);
  const dir=process.env.JUDEX_E2E_ARTIFACT??'.cache/demo4-implementation/business';fs.mkdirSync(dir,{recursive:true});await a.goto(`/projects/${project.id}/plans/${plan.id}/route`);await a.screenshot({path:dir+'/demo4-business-route.png',fullPage:true,animations:'disabled'});
 }finally{await f.ca.close();await f.cb.close();}
});
test('D4-B03 formal edits keep current arrangement until every responsibility agrees',async({browser})=>{
 const f=await setupCooperation(browser),{a,b,root,task,plan,foreign,identity}=f;
 try{
  await a.goto(`${root}/plans/${plan.id}/route?task=${task.id}`);await a.getByTestId('task-inspector').getByTestId('task-more-'+task.id).click();await a.getByRole('menuitem',{name:'修改任务安排',exact:true}).click();await a.getByTestId('edit-work-description').fill('修改后的成果要求须各职责明确同意');await a.getByTestId('edit-participant-'+foreign.id).click();await a.getByTestId('save-work-edit').click();await expect(a.locator('.judex-task-compare')).toContainText('登录实现与测试证据');await expect(a.locator('.judex-task-compare')).toContainText('修改后的成果要求');await a.getByTestId('save-work-edit').click();await expect(a).toHaveURL(/tab=decisions/);
  const pending=(await command(a,'get',root+'/proposals')).items.find((p:any)=>p.kind==='work_change'&&p.status==='pending');expect(pending).toBeTruthy();let current=await command(a,'get',root+'/tasks/'+task.id);expect(current.expectedOutput).toBe(task.expectedOutput);expect(current.participants).toHaveLength(1);
  await approveCooperation(a,root,pending.id);expect((await command(a,'get',root+'/tasks/'+task.id)).expectedOutput).toBe(task.expectedOutput);await approveCooperation(b,root,pending.id);current=await command(a,'get',root+'/tasks/'+task.id);expect(current.expectedOutput).toBe('修改后的成果要求须各职责明确同意');expect(current.participants.map((p:any)=>p.identityId).sort()).toEqual([identity.id,foreign.id].sort());expect(current.reviewerIdentityId).toBe(identity.id);
 }finally{await f.ca.close();await f.cb.close();}
});
test('D4-B04 CLI exception needs one human confirmation and context preserves the fact',async({browser},info)=>{
 const f=await setupCooperation(browser),{a,root,project,task}=f;
 try{
  const scopes=['projects:read','context:read','intents:create'];const device=await command(a,'post','/auth/device/authorizations',{deviceName:'Demo4 CLI',requestedScopes:scopes,projectScope:[project.id]});await command(a,'post','/auth/device/confirm',{userCode:device.userCode,approved:true,scopes,projectScope:[project.id]});const creds=await command(a,'post','/auth/device/token',{deviceCode:device.deviceCode});
  const config=path.join(process.env.JUDEX_E2E_ARTIFACT??info.outputDir,'demo4-cli');fs.mkdirSync(config,{recursive:true});const run=(args:string[])=>JSON.parse(String(execFileSync(process.env.JUDEX_E2E_CLI!,['--server',process.env.JUDEX_E2E_BASE_URL!,'--project',project.id,'--json','--no-wait',...args],{encoding:'utf8',windowsHide:true,env:{...process.env,JUDEX_TOKEN:creds.accessToken,JUDEX_CONFIG_DIR:config}}))).data;
  const preview=run(['task','skip',task.id]);expect(preview.operation).toBe('skip');expect((await command(a,'get',root+'/tasks/'+task.id)).executionException).toBeNull();const intent=run(['task','skip',task.id,'--reason','由管理员核对本次运行例外']);expect(intent.operation).toBe('task.skip');await a.goto(intent.confirmUrl);await expect(a.getByTestId('confirm-approve')).toBeEnabled();await expect(a.getByTestId('fixed-review')).toContainText('由管理员核对本次运行例外');await a.getByTestId('confirm-approve').click();await expect.poll(()=>run(['decision','result',intent.id]).state).toBe('committed');
  await command(a,'post',root+`/confirmation-intents/${intent.id}/confirm`,{decision:'approve',intentHash:intent.nonce});const context=run(['context','get',task.id]);expect(context.executionException.reason).toContain('管理员');expect(context.status).toBe('ready');expect(context.capabilities.skip).toBe(false);expect(context.capabilities.restore).toBe(true);
  const audit=(await command(a,'get',root+'/audit')).items.filter((r:any)=>r.operation==='task.execution.skip'&&r.objectId===task.id);expect(audit).toHaveLength(1);expect(audit[0].source).toBe('cli');
  const resume=run(['task','restore',task.id,'--reason','继续完成原任务']);await a.goto(resume.confirmUrl);await a.getByTestId('confirm-approve').click();await expect.poll(()=>run(['decision','result',resume.id]).state).toBe('committed');expect(run(['context','get',task.id]).executionException).toBeNull();fs.writeFileSync(path.join(config,'context-facts.json'),JSON.stringify({preview,context},null,2));
 }finally{await f.ca.close();await f.cb.close();}
});
test('D4-B02 real draft discard freezes child list and preserves history',async({browser})=>{
 const f=await setupCooperation(browser),{a,b,root,identity}=f;
 try{
  const plan=await command(a,'post',root+'/plans',{title:'可丢弃草稿计划',ownerIdentityId:identity.id});const task=await command(a,'post',root+'/tasks',{planId:plan.id,title:'保留原记录的草稿'});
  const review=await command(a,'get',root+`/plans/${plan.id}/discard-review`);expect(review.tasks).toHaveLength(1);const next=await command(a,'post',root+'/tasks',{planId:plan.id,title:'新增草稿'});
  const session=(await(await a.request.get('/api/v1/auth/session')).json()).data;const stale=await a.request.post('/api/v1'+root+`/plans/${plan.id}/discard`,{data:{expectedVersion:plan.version,reviewHash:review.reviewHash},headers:{'X-CSRF-Token':session.csrfToken,'Idempotency-Key':crypto.randomUUID()}});expect(stale.status()).toBe(409);
  const current=await command(a,'get',root+`/plans/${plan.id}/discard-review`);await command(a,'post',root+`/plans/${plan.id}/discard`,{expectedVersion:plan.version,reviewHash:current.reviewHash});expect((await command(a,'get',root+'/plans')).items.some((p:any)=>p.id===plan.id)).toBe(false);expect((await command(a,'get',root+'/tasks/'+task.id)).discardedAt).toBeTruthy();expect((await command(a,'get',root+'/tasks/'+next.id)).discardedAt).toBeTruthy();expect((await command(a,'get',root+'/plans/'+plan.id)).mainTopicId).toBe(plan.mainTopicId);
 }finally{await f.ca.close();await f.cb.close();}
});
test('D4-B05 a member proposes draft changes without acquiring direct edit permission',async({browser})=>{
 const f=await setupCooperation(browser),{a,b,root,identity,foreign,flow}=f;try{
  const plan=await command(a,'post',root+'/plans',{title:'成员提议的草稿修改',ownerIdentityId:identity.id}),task=await command(a,'post',root+'/tasks',{planId:plan.id,title:'作者保存的草稿',expectedOutput:'原草稿说明',participantIdentityIds:[identity.id,foreign.id],reviewerIdentityId:identity.id,workflowId:flow.id,nodeId:'work'});
  const old=await command(a,'post',root+'/proposals',{kind:'work_arrangement',changes:[{operation:'activate_object',targetType:'task',targetId:task.id,expectedVersion:task.version}]}),oldInitial=await command(a,'get',root+'/proposals/'+old.id+'/review');
  await command(a,'post',root+'/proposals/'+old.id+'/submit',{expectedVersion:old.version,draftHash:oldInitial.reviewHash});await approveCooperation(a,root,old.id);expect((await command(a,'get',root+'/proposals/'+old.id+'/review')).status).toBe('pending');
  await b.goto(`${root}/plans/${plan.id}/route?task=${task.id}`);await b.getByTestId('task-inspector').getByTestId('task-more-'+task.id).click();await expect(b.getByRole('menuitem',{name:'编辑草稿任务',exact:true})).toHaveCount(0);await b.getByRole('menuitem',{name:'修改任务安排',exact:true}).click();await b.getByTestId('edit-work-description').fill('成员提出的新草稿说明');await b.getByTestId('save-work-edit').click();await expect(b.locator('.judex-task-compare')).toContainText('原草稿说明');await b.getByTestId('save-work-edit').click();await expect(b).toHaveURL(/tab=decisions/);expect((await command(a,'get',root+'/tasks/'+task.id)).expectedOutput).toBe('原草稿说明');const pending=(await command(a,'get',root+'/proposals')).items.find((p:any)=>p.kind==='work_change'&&p.status==='pending');await approveCooperation(a,root,pending.id);await approveCooperation(b,root,pending.id);const updated=await command(a,'get',root+'/tasks/'+task.id);expect(updated.expectedOutput).toBe('成员提出的新草稿说明');expect(updated.status).toBe('draft');expect(updated.capabilities.editDraft).toBe(true);expect((await command(b,'get',root+'/tasks/'+task.id)).capabilities.editDraft).toBe(false);const withdrawn=await command(a,'get',root+'/proposals/'+old.id+'/review');expect(withdrawn.status).toBe('cancelled');expect(withdrawn.slots.some((s:any)=>s.state==='approved')).toBe(true);expect((await command(a,'get',root+'/proposals/'+pending.id+'/review')).status).toBe('approved');
 }finally{await f.ca.close();await f.cb.close();}
});
test('D4-B06 plan acceptance displays frozen skip basis and remains an explicit owner decision',async({browser})=>{
 const f=await setupCooperation(browser),{a,root,task,plan}=f;try{
  const review=await command(a,'get',root+`/tasks/${task.id}/execution-review?operation=skip`);await command(a,'post',root+`/tasks/${task.id}/skip`,{expectedVersion:review.targetVersion,reviewHash:review.reviewHash,reason:'现有成果已覆盖计划目标，保留本次跳过依据',waivers:[]});expect((await command(a,'get',root+'/plans/'+plan.id)).status).toBe('active');await a.goto(`${root}/plans/${plan.id}/route`);await a.getByTestId('accept-plan').click();await expect(a.locator('.judex-dialog-content')).toContainText('现有成果已覆盖计划目标，保留本次跳过依据');await expect(a.locator('.judex-dialog-content')).toContainText('已跳过');await a.getByTestId('confirm-final-acceptance').click();await expect(a.locator('.judex-dialog-content')).toHaveCount(0);expect((await command(a,'get',root+'/plans/'+plan.id)).status).toBe('accepted');const current=await command(a,'get',root+'/tasks/'+task.id);expect(current.status).toBe('ready');expect(current.executionException.reason).toContain('覆盖计划目标');expect((await command(a,'get',root+'/plans/'+plan.id)).taskStats.accepted).toBe(0);
 }finally{await f.ca.close();await f.cb.close();}
});

test('D4-B07 real published rules resolve fixed material and handoff sources in the shared viewer', async ({browser}, info) => {
 const f = await setupCooperation(browser), {a, b, root, project, task, identity, foreign, flow: initialFlow} = f;
 try {
  const bytes = Buffer.from('D4_FIXED_MATERIAL: 核对任务与交接来源的固定依据。'), sha = createHash('sha256').update(bytes).digest('hex');
  const upload = await command(a, 'post', root + '/uploads', {name: 'D4-固定依据.md', size: bytes.length, sha256: sha, mime: 'text/markdown', kind: 'file', purpose: '流程规则使用的固定材料'});
  const session = await command(a, 'get', '/auth/session');
  for (let i = 0; i < upload.partCount; i++) {
   const part = bytes.subarray(i * upload.partSize, (i + 1) * upload.partSize);
   const response = await a.request.put('/api/v1' + root + `/uploads/${upload.id}/parts/${i + 1}`, {data: part, headers: {'X-CSRF-Token': session.csrfToken, 'X-Judex-Part-SHA256': createHash('sha256').update(part).digest('hex')}});
   expect(response.ok(), await response.text()).toBe(true);
  }
  const version = await command(a, 'post', root + `/uploads/${upload.id}/complete`, {});
  const report = await command(a, 'post', root + `/tasks/${task.id}/reports`, {kind: 'delivery', text: '已提供真实来源交付', identityId: identity.id, expectedTaskVersion: task.version});
  const handoff = await command(a, 'post', root + '/handoffs', {title: '真实规则来源交接', kind: 'stage', targetTaskId: task.id, receiverIdentityId: foreign.id, sources: [{sourceTaskId: task.id, senderIdentityId: identity.id}]});
  const contribution = (await command(a, 'get', root + '/handoffs')).items.find((h: any) => h.id === handoff.id).sources[0];
  await command(a, 'post', root + `/handoffs/${handoff.id}/sources/${contribution.id}/send`, {summary: '真实固定交付依据', sourceVersion: (contribution.currentVersion ?? 0) + 1, reviewHash: report.id});
  const standalone = await command(a, 'post', root + '/tasks', {title: '没有所属计划的来源任务', workflowId: initialFlow.id, nodeId: 'work', participantIdentityIds: [identity.id], reviewerIdentityId: identity.id});
  const body = {name: '三类真实依据流程', instructions: '全局说明与环节职责必须分别保留', nodes: [{id: 'rules', name: '真实依据核对', responsibility: '按来源核对材料、任务与交接', allowedPositionIds: [], defaultApprovalPolicy: 'all'}], advisoryEdges: [], hardRules: [{nodeId: 'rules', kind: 'task_acceptance', phase: 'start', targetId: task.id}, {nodeId: 'rules', kind: 'material_ready', phase: 'accept', targetId: version.id}, {nodeId: 'rules', kind: 'handoff_receipt', phase: 'both', targetId: contribution.id}, {nodeId: 'rules', kind: 'task_acceptance', phase: 'accept', targetId: standalone.id}]};
  const flow = await command(a, 'post', root + '/workflows', body), draft = (await command(a, 'get', root + `/workflows/${flow.id}/versions`)).items[0];
  await command(a, 'post', root + `/workflows/${flow.id}/publish`, {expectedVersion: flow.version, draftHash: draft.draftHash});
  const role = await command(a, 'post', root + '/positions', {name: '规则核对职责', nodeBindings: [{workflowId: flow.id, nodeId: 'rules'}]}), seat = await command(a, 'post', root + '/identities', {positionId: role.id, userId: session.user.id});
  const plan = await command(a, 'post', root + '/plans', {title: '真实来源查看计划', ownerIdentityId: seat.id, workflowId: flow.id}), target = await command(a, 'post', root + '/tasks', {title: '三类规则任务', planId: plan.id, workflowId: flow.id, nodeId: 'rules', participantIdentityIds: [seat.id], reviewerIdentityId: seat.id});
  // Start from a cold hub, whose workbench has not loaded workflow versions or
  // identity metadata. Preview must still show the real published reference.
  await a.goto(root);
  const card = a.locator('.judex-co-plan-card').filter({hasText: '真实来源查看计划'});
  await card.getByRole('button', {name: '预览路线', exact: true}).click();
  await a.getByTestId('route-fullscreen').getByTestId('execution-task-' + target.id).locator('.judex-co-card-hit').click();
  await a.getByTestId('route-fullscreen').getByRole('tab', {name: '流程参考', exact: true}).click();
  await expect(a.getByTestId('route-fullscreen').getByTestId('task-section-flow')).toContainText('三类真实依据流程');
  await expect(a.getByTestId('route-fullscreen').locator('[data-rule-kind="material_ready"]')).toContainText('D4-固定依据.md');
  await a.getByTestId('route-fullscreen').getByTestId('exit-route-fullscreen').click();
  await a.goto(`${root}/plans/${plan.id}/route?task=${target.id}&section=flow`);
  const reference = a.getByTestId('task-section-flow');
  await expect(reference).toContainText('全局说明与环节职责必须分别保留');
  await expect(reference.locator('[data-rule-target="' + task.id + '"]')).toContainText('登录功能');
  await expect(reference.locator('[data-rule-kind="material_ready"]')).toContainText('D4-固定依据.md');
  await expect(reference.locator('[data-rule-kind="material_ready"]')).toContainText('固定材料版本 · v1');
  await expect(reference.locator('[data-rule-kind="handoff_receipt"]')).toContainText('真实规则来源交接');
  await expect(reference.locator('[data-rule-kind="handoff_receipt"]')).toContainText('交付 v1');
  await expect(reference.locator('[data-rule-kind="handoff_receipt"]')).toContainText('待接收');
  await reference.locator('[data-rule-kind="material_ready"]').getByRole('button').click();
  await expect(a.getByRole('dialog')).toContainText('D4-固定依据.md');
  await a.getByRole('dialog').locator('[data-slot="modal-close-trigger"]').click();
  await reference.getByTestId('locate-task-plan').click();await expect(a.getByTestId('route-fullscreen')).toBeVisible();
  await a.getByTestId('route-fullscreen').getByRole('tab', {name: '流程参考', exact: true}).click();
  await a.getByTestId('route-fullscreen').locator('.judex-task-scroll').evaluate(el => {el.scrollTop = el.scrollHeight;});
  await a.screenshot({path: info.outputPath('real-flow-rule-sources.png'), fullPage: true});
  await a.getByTestId('route-fullscreen').locator('[data-rule-kind="handoff_receipt"]').getByRole('button').click();
  await expect(a.getByTestId('route-fullscreen')).toHaveCount(0);await expect(a).toHaveURL(new RegExp('item=' + handoff.id));
  await expect(a.getByRole('dialog')).toContainText('真实规则来源交接');
  await expect(a.getByTestId('contribution-' + contribution.id)).toHaveClass(/judex-contribution-focused/);
  await a.goto(`${root}/plans/${plan.id}/route?task=${target.id}&section=flow`);
  await a.getByTestId('task-section-flow').getByTestId('locate-task-plan').click();
  await a.getByTestId('route-fullscreen').getByRole('tab', {name: '流程参考', exact: true}).click();
  await a.getByTestId('route-fullscreen').locator('[data-rule-target="' + standalone.id + '"]').getByRole('button').click();
  await expect(a.getByTestId('route-fullscreen')).toHaveCount(0);
  await expect(a.getByTestId('task-details-drawer')).toContainText('没有所属计划的来源任务');
  await expect(a).toHaveURL(new RegExp('item=' + standalone.id));
  expect((await command(b, 'get', root + `/tasks/${target.id}`)).status).toBe('draft');
  await info.attach('published-reference-proof', {body: JSON.stringify({workflow: flow.id, task: target.id, materialVersion: version.id, handoff: handoff.id, source: contribution.id, phases: body.hardRules.map(r => r.phase)}), contentType: 'application/json'});
 } finally {await f.ca.close(); await f.cb.close();}
});
