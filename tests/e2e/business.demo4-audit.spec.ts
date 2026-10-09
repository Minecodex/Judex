import {test, expect} from '@playwright/test';
import {setupCooperation, command} from './cooperation-business-helpers';

test('D4-R5 external task links load by id in cold and warm preview, route and discussion', async ({browser}, info) => {
  const f = await setupCooperation(browser), {a, root, task: external, flow, identity} = f;
  try {
    const plan = await command(a, 'post', root + '/plans', {title: '外部前置导航验收', ownerIdentityId: identity.id, workflowId: flow.id});
    const task = await command(a, 'post', root + '/tasks', {title: '当前计划任务', planId: plan.id, workflowId: flow.id, nodeId: 'work', participantIdentityIds: [identity.id], reviewerIdentityId: identity.id, requirements: [{phase: 'start', kind: 'task_acceptance', targetId: external.id, hard: true, label: '跨计划前置'}]});
    const graph = await command(a, 'get', root + '/execution-map?planId=' + plan.id + '&includeDrafts=true&includeReferences=true');
    expect(graph.edges.some((e: any) => e.fromTaskId === external.id && e.toTaskId === task.id)).toBe(true);
    let reads = 0;
    a.on('request', req => {if (new URL(req.url()).pathname.endsWith('/tasks/' + external.id) && req.method() === 'GET') reads++;});
    await a.goto(root);
    await a.getByTestId('plan-preview-' + plan.id).click();
    const viewer = a.getByTestId('route-fullscreen');
    const chip = viewer.getByTestId('execution-task-' + task.id).locator('.judex-co-external-links button').first();
    await chip.click();
    await expect(viewer.locator('.judex-task-title')).toContainText('登录功能');
    await expect(viewer.locator('.judex-task-breadcrumb')).toContainText(f.plan.title);
    await expect(viewer.getByTestId('start-task')).toHaveCount(0);
    await expect(viewer.getByRole('button', {name: '编辑草稿任务', exact: true})).toHaveCount(0);
    expect(reads).toBeGreaterThan(0);
    await a.screenshot({path: info.outputPath('external-task-loaded.png'), fullPage: true});
    await viewer.getByTestId('close-task-drawer').click();await expect(chip).toBeFocused();
    await viewer.getByTestId('execution-task-' + task.id).locator('.judex-co-card-hit').click();
    await viewer.getByTestId('task-section-overview').locator('.judex-task-condition').filter({hasText: '跨计划前置'}).getByRole('button').click();
    await expect(viewer.locator('.judex-task-title')).toContainText('登录功能');
    await expect(viewer).toBeVisible();
    await viewer.getByTestId('exit-route-fullscreen').click();
    await a.goto(`${root}/plans/${plan.id}/route?task=${task.id}`);
    await a.getByTestId('task-section-overview').locator('.judex-task-condition').filter({hasText: '跨计划前置'}).getByRole('button').click();
    await expect(a.locator('.judex-task-title')).toContainText('登录功能');
    await expect(a).toHaveURL(new RegExp('task=' + external.id));
    await a.goto(`${root}/plans/${plan.id}/chat/${plan.mainTopicId}?taskContext=${task.id}`);
    await a.getByTestId('work-discussion-input').fill('保留原讨论及本次发言任务');
    await a.locator('.judex-co-context-task').filter({hasText: '当前计划任务'}).click();
    await a.getByTestId('task-section-overview').locator('.judex-task-condition').filter({hasText: '跨计划前置'}).getByRole('button').click();
    await expect(a.getByTestId('task-inspector').locator('.judex-task-title')).toContainText('登录功能');
    await expect(a.getByTestId('work-discussion-input')).toHaveValue('保留原讨论及本次发言任务');
    await expect(a).toHaveURL(new RegExp('chat/' + plan.mainTopicId));
    expect(new URL(a.url()).searchParams.get('taskContext')).toBe(task.id);
    await info.attach('external-link-verification', {body: JSON.stringify({project:f.project.id,plan:plan.id,task:task.id,external:external.id,detailReads:reads,previewReadOnly:true,discussionContextPreserved:true}),contentType:'application/json'});
  } finally {await f.ca.close();await f.cb.close();}
});

test('D4-R6 real blocked status agrees across route, inspector and discussion without changing the phase', async ({browser}, info) => {
  const f=await setupCooperation(browser),{a,b,root,flow,identity}=f;
  try {
    const task=await command(a,'post',root+'/tasks',{title:'真实等待前置任务',planId:f.plan.id,workflowId:flow.id,nodeId:'work',participantIdentityIds:[identity.id],reviewerIdentityId:identity.id,requirements:[{phase:'start',kind:'task_acceptance',targetId:f.task.id,hard:true,label:'原任务必须验收'}]});
    const draft=await command(a,'post',root+'/proposals',{kind:'work_change',reason:'阻塞状态验收',changes:[{operation:'activate_object',targetType:'task',targetId:task.id,expectedVersion:task.version}]});
    const review=await command(a,'get',root+'/proposals/'+draft.id+'/review');
    await command(a,'post',root+'/proposals/'+draft.id+'/submit',{expectedVersion:draft.version,draftHash:review.reviewHash});
    const {approveCooperation}=await import('./cooperation-business-helpers');await approveCooperation(a,root,draft.id);await approveCooperation(b,root,draft.id);
    await a.goto(`${root}/plans/${f.plan.id}/route?task=${task.id}`);
    await expect(a.getByTestId('execution-task-'+task.id)).toHaveAttribute('data-stage','blocked');
    await expect(a.getByTestId('task-inspector').locator('.judex-task-status')).toHaveText('等待前置');
    await expect(a.getByTestId('start-task')).toBeDisabled();
    await a.screenshot({path:info.outputPath('consistent-blocked-status.png'),fullPage:true});
    await a.goto(`${root}/plans/${f.plan.id}/chat/${f.plan.mainTopicId}`);
    await expect(a.locator('.judex-co-context-task').filter({hasText:'真实等待前置任务'}).locator('.judex-task-status')).toHaveText('等待前置');
    const current=await command(a,'get',root+'/tasks/'+task.id);expect(current.status).toBe('ready');expect(current.version).toBe(task.version+1);
    await info.attach('blocked-display-facts',{body:JSON.stringify({id:task.id,status:current.status,version:current.version,display:'blocked',startEnabled:false}),contentType:'application/json'});
  } finally {await f.ca.close();await f.cb.close();}
});

test('D4-R5 task lookup failure keeps its selection and offers a successful retry', async ({browser}) => {
 const f=await setupCooperation(browser),{a,root,task}=f;
 try {
  const endpoint='**/api/v1'+root+'/tasks/'+task.id;
  await a.route(endpoint,route=>route.fulfill({status:503,json:{error:{code:'SERVICE_UNAVAILABLE',message:'temporary fixture outage',retryable:true}}}));
  await a.goto(root);await a.getByTestId('plan-preview-'+f.plan.id).click();
  const viewer=a.getByTestId('route-fullscreen');
  await viewer.getByTestId('execution-task-'+task.id).locator('.judex-co-card-hit').click();
  const drawer=viewer.getByTestId('task-details-drawer');
  await expect(drawer.getByRole('button',{name:'重新连接',exact:true})).toBeVisible();
  await expect(drawer.getByTestId('task-inspector')).toHaveCount(0);
  await a.unroute(endpoint);await drawer.getByRole('button',{name:'重新连接',exact:true}).click();
  await expect(drawer.locator('.judex-task-title')).toContainText('登录功能');
  await expect(viewer).toBeVisible();
 } finally {await f.ca.close();await f.cb.close();}
});
