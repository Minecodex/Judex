import {expect, test} from '@playwright/test';
import {execFileSync, spawnSync} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

// An explicit live experiment: never counted as controlled/demo acceptance.
test('WR01 live CLI reports with one position bound to three workflows', async ({browser, baseURL}, info) => {
  test.skip(process.env.JUDEX_ROUTING_LIVE !== '1', 'Explicit owned real-model experiment only');
  test.setTimeout(1800000);
  const artifact = process.env.JUDEX_E2E_ARTIFACT!;
  const context = await browser.newContext();
  const baseline: any = {cases: [], uploads: [], failures: []};
  try {
    const page = await context.newPage();
    await page.goto('/register');
    await page.getByLabel(/名称|Name/).fill('多流程实验成员');
    await page.getByLabel(/邮箱|Email/).fill(`routing-${randomUUID()}@routing.test`);
    await page.getByLabel(/密码|Password/).first().fill('owned-routing-fixture-password-123');
    await page.getByLabel(/确认密码|Confirm/).fill('owned-routing-fixture-password-123');
    await page.getByRole('button', {name: /创建账号|Create account/}).click();
    await expect(page.getByTestId('workspace-new-project')).toBeVisible();
    const session = (await (await page.request.get('/api/v1/auth/session')).json()).data;
    const api = async (method: 'get' | 'post' | 'put', route: string, data?: unknown) => {
      const response = method === 'get' ? await page.request.get('/api/v1' + route) :
        await page.request[method]('/api/v1' + route, {data, headers: {'X-CSRF-Token': session.csrfToken, 'Idempotency-Key': randomUUID()}});
      expect(response.ok(), await response.text()).toBeTruthy();
      return (await response.json()).data;
    };
    const project = await api('post', '/projects', {title: '多流程内容判定实验', maxDiscussionRounds: 5});
    const root = '/projects/' + project.id;
    baseline.projectId = project.id;
    const descriptions = [
      {key: 'software', name: '软件功能交付', instructions: '适用于计划中的新增功能或缺陷修复交付，包括实现与本地联调、交付证据自检。不处理生产事故恢复或独立性能实验。',
        nodes: [{id: 'work', name: '功能实现与联调', responsibility: '实现计划约定的功能或修复并记录本地联调结果。'},
          {id: 'verify', name: '功能交付证据核对', responsibility: '核对功能实现的测试和交付证据，保留缺口，最终验收仍由有权人执行。'}]},
      {key: 'incident', name: '线上事故处理', instructions: '适用于已经影响生产用户的故障。先止损恢复服务，再调查根因与防复发整改。监控告警、用户影响、生产环境和恢复操作是关键依据。',
        nodes: [{id: 'work', name: '事故止损与服务恢复', responsibility: '核对生产故障影响、回滚或降级、服务恢复证据。'},
          {id: 'verify', name: '事故根因与整改核对', responsibility: '调查已发生事故的根因、整改及防复发验证，不能把压测实验当事故。'}]},
      {key: 'performance', name: '性能优化验证', instructions: '适用于预先计划的性能实验。使用隔离压测环境、固定并发、基线与优化后指标作对照。没有生产用户事故时，不触发事故恢复。',
        nodes: [{id: 'work', name: '压测基线采集', responsibility: '采集固定环境和负载下的吞吐、延迟、错误率基线。'},
          {id: 'verify', name: '优化对照验证', responsibility: '在同一负载与环境对照优化前后的指标，核对样本、阈值和可复现性。'}]},
    ];
    const workflows: any[] = [];
    for (const description of descriptions) {
      const body = {name: description.name, instructions: description.instructions + '\n上报分析应结合当前工作事实，必要时调用当前工程协作身份核对；审批与验收由人执行。',
        nodes: description.nodes.map(node => ({...node, allowedPositionIds: [], defaultApprovalPolicy: 'all'})), advisoryEdges: [{from: 'work', to: 'verify'}]};
      const head = await api('post', root + '/workflows', body);
      const draft = (await api('get', root + `/workflows/${head.id}/versions`)).items.find((item: any) => item.state === 'draft');
      await api('post', root + `/workflows/${head.id}/publish`, {expectedVersion: head.version, draftHash: draft.draftHash});
      workflows.push({key: description.key, id: head.id, body});
    }
    const bindings = workflows.flatMap(flow => flow.body.nodes.map((node: any) => ({workflowId: flow.id, nodeId: node.id})));
    const position = await api('post', root + '/positions', {name: '工程协作岗', prompt: '核对工程协作中的上报事实、证据缺口、适用职责及协作建议。结合节点职责与实际工作背景；无法确定时保留异议并请求补充。不得批准或验收。', nodeBindings: bindings});
    const identity = await api('post', root + '/identities', {positionId: position.id, userId: session.user.id});
    for (const flow of workflows) {
      const head = (await api('get', root + '/workflows')).items.find((item: any) => item.id === flow.id);
      flow.body.nodes = flow.body.nodes.map((node: any) => ({...node, allowedPositionIds: [position.id]}));
      await api('put', root + `/workflows/${flow.id}/draft`, {expectedVersion: head.version, body: flow.body});
      const updated = (await api('get', root + '/workflows')).items.find((item: any) => item.id === flow.id);
      const draft = (await api('get', root + `/workflows/${flow.id}/versions`)).items.find((item: any) => item.state === 'draft');
      const published = await api('post', root + `/workflows/${flow.id}/publish`, {expectedVersion: updated.version, draftHash: draft.draftHash});
      flow.publishedVersionId = published.id;
    }
    const savedPosition = (await api('get', root + '/positions')).items.find((item: any) => item.id === position.id);
    expect(savedPosition.nodeBindings).toHaveLength(6);
    baseline.positionId = position.id;
    baseline.identityId = identity.id;
    baseline.positionBindings = savedPosition.nodeBindings;
    const workflow = (key: string) => workflows.find(flow => flow.key === key)!;
    const cases: any[] = [
      {id: 'bound-software', title: '修复重复登录回调', goal: '交付新版登录功能', flow: 'software', node: 'work', text: '已修复重复登录回调，在本地开发环境通过单元测试与联调，尚缺跨浏览器验证。'},
      {id: 'bound-incident', title: '恢复生产订单接口', goal: '恢复生产订单服务并核对用户影响', flow: 'incident', node: 'work', text: '生产订单接口出现大量502，已回滚最近的配置变更，错误率恢复正常，附恢复过程和监控采样。'},
      {id: 'bound-performance', title: '缓存优化同负载对照', goal: '在隔离环境验证缓存优化收益', flow: 'performance', node: 'verify', text: '隔离压测环境、固定200并发，优化前P95为620ms，优化后为210ms。尚缺重复采样与错误率对照。'},
      {id: 'plan-only', title: '核对实验材料', goal: '在隔离压测环境验证缓存优化前后性能', planFlow: 'performance', text: '两轮对照样本已经采集，正在核对负载与环境是否一致，缺少第三轮重复测量。'},
      {id: 'content-only', title: '工程问题核对', goal: '核对本次工程上报的事实和缺口', text: '生产支付服务在10:20出现超时，已通过降级恢复，错误率由18%回落至0.2%，需要核对止损证据。'},
    ];
    const changes: any[] = [];
    for (const item of cases) {
      changes.push({operation: 'create_plan', targetType: 'plan', clientRef: 'p-' + item.id, fields: {title: item.goal, goal: item.goal,
        acceptanceCriteria: '依据事实与证据由有权人验收', ownerIdentityId: identity.id, ...((item.flow || item.planFlow) ? {workflowId: workflow(item.flow || item.planFlow).id} : {})}});
      changes.push({operation: 'create_task', targetType: 'task', clientRef: item.id, fields: {title: item.title, expectedOutput: item.text,
        acceptanceCriteria: '证据可核对，保留未完成项', participantIdentityIds: [identity.id], reviewerIdentityId: identity.id, planId: 'p-' + item.id,
        ...(item.flow ? {workflowId: workflow(item.flow).id, nodeId: item.node} : {})}});
    }
    const proposal = await api('post', root + '/proposals', {kind: 'work_arrangement', reason: '本轮隔离实验的明确测试安排', changes});
    const initial = await api('get', root + `/proposals/${proposal.id}/review`);
    await api('post', root + `/proposals/${proposal.id}/submit`, {expectedVersion: proposal.version, draftHash: initial.reviewHash});
    const review = await api('get', root + `/proposals/${proposal.id}/review`);
    await api('post', root + `/proposals/${proposal.id}/decisions`, {decision: 'approve', expectedVersion: 2, reviewId: review.reviewId, reviewHash: review.reviewHash,
      slotIds: review.slots.filter((slot: any) => slot.canDecide).map((slot: any) => slot.id),
      actingBindingVersions: review.slots.filter((slot: any) => slot.canDecide && slot.authorityType === 'identity').map((slot: any) => ({identityId: slot.authorityId, bindingVersion: slot.bindingVersion}))});
    const scopes = ['projects:read', 'context:read', 'materials:read', 'materials:write', 'submissions:write', 'reports:write', 'events:read'];
    const authorizeCLI = async () => {
      const device = await api('post', '/auth/device/authorizations', {deviceName: 'owned-workflow-routing-cli', requestedScopes: scopes, projectScope: [project.id]});
      await api('post', '/auth/device/confirm', {userCode: device.userCode, approved: true, scopes, projectScope: [project.id]});
      return await api('post', '/auth/device/token', {deviceCode: device.deviceCode});
    };
    let credentials = await authorizeCLI();
    const cli = (args: string[]) => JSON.parse(String(execFileSync(process.env.JUDEX_E2E_CLI!, ['--server', baseURL!, '--project', project.id, '--no-wait', ...args],
      {encoding: 'utf8', windowsHide: true, timeout: 60000, env: {...process.env, JUDEX_TOKEN: credentials.accessToken}})));
    expect(cli(['auth', 'whoami']).user.id).toBe(session.user.id);
    const fixture = {schemaVersion: 1, project: {id: project.id, title: project.title}, position: savedPosition,
      workflows: workflows.map(flow => ({id: flow.id, publishedVersionId: flow.publishedVersionId, ...flow.body})),
      cases: inferenceCases(workflow)};
    fs.writeFileSync(path.join(artifact, 'routing-fixture.json'), JSON.stringify(fixture, null, 2));
    const selected = new Set((process.env.JUDEX_ROUTING_BASELINE_CASE_IDS ?? '').split(',').filter(Boolean));
    const selectedCases = cases.filter(item => !selected.size || selected.has(item.id));
    expect(selectedCases.length).toBeGreaterThan(0);
    baseline.selectedCases = selectedCases.map(item => item.id);
    for (const item of selectedCases) {
      // Each fixture command group gets a fresh grant. A one-shot JUDEX_TOKEN
      // cannot use the normal persisted client's refresh token after 15 minutes.
      credentials = await authorizeCLI();
      const summary = (await api('get', root + '/tasks')).items.find((task: any) => task.title === item.title);
      const task = await api('get', root + `/tasks/${summary.id}`);
      const plan = await api('get', root + `/plans/${task.planId}`);
      await api('post', root + `/tasks/${task.id}/start`, {expectedVersion: task.version, identityId: identity.id});
      const current = await api('get', root + `/tasks/${task.id}`);
      const file = path.join(artifact, item.id + '-report.json');
      const sourceText = item.text + '\n请结合当前计划、任务和已发布流程核对这份上报，说明适用环节与证据缺口。需要时调用当前工程协作身份，并保存公开分析。';
      let materials: string[] = [];
      if (item.id === 'bound-performance') {
        const materialPath = path.join(artifact, 'performance-evidence.txt');
        fs.writeFileSync(materialPath, '环境：隔离压测；并发：200；优化前P95：620ms；优化后P95：210ms；第三轮采样和错误率记录缺失。');
        const upload = cli(['material', 'upload', materialPath]);
        const versionId = upload.versionId ?? upload.materialVersionId ?? upload.id;
        expect(versionId).toMatch(/^[0-9a-f-]{36}$/);
        materials = [versionId]; baseline.uploads.push(upload);
      }
      const payload = {text: sourceText, expectedTaskVersion: current.version, identityId: identity.id, materialVersionIds: materials};
      expect(Object.keys(payload)).not.toContain('workflowId'); expect(Object.keys(payload)).not.toContain('nodeId');
      fs.writeFileSync(file, JSON.stringify(payload, null, 2));
      const receipt = cli(['report', '--task', task.id, '--kind', 'progress', '--file', file]);
      const record: any = {id: item.id, taskId: task.id, planId: task.planId, taskWorkflowId: task.workflowId, taskNodeId: task.nodeId,
        planWorkflowId: plan.workflowId, input: payload, receipt};
      baseline.cases.push(record);
      console.log('Awaiting real platform analysis: ' + item.id);
      let analysis: any;
      try {
        await expect.poll(async () => {
          const activity = (await api('get', root + `/tasks/${task.id}/activity`)).items.find((row: any) => row.text === sourceText);
          record.sourceId = activity?.id; record.source = activity?.source; analysis = activity?.analysis;
          return ['completed', 'failed', 'waiting_human', 'cancelled'].includes(analysis?.state);
        }, {timeout: 300000, intervals: [1000, 2000, 4000]}).toBe(true);
        expect(analysis?.state).toBe('completed');
        expect(record.source).toBe('cli');
      } catch (error: any) { baseline.failures.push({id: item.id, error: error.message}); }
      record.analysis = analysis;
      record.taskAfter = await api('get', root + `/tasks/${task.id}`);
      expect(record.taskAfter.status).toBe('working');
      expect(record.taskAfter.workflowId).toBe(task.workflowId);
      expect(record.taskAfter.nodeId).toBe(task.nodeId);
      fs.writeFileSync(path.join(artifact, 'platform-baseline.json'), JSON.stringify(baseline, null, 2));
      console.log('Platform analysis result: ' + item.id + ' ' + analysis?.state);
    }
    const freeFile = path.join(artifact, 'unlinked-submission.json');
    credentials = await authorizeCLI();
    const freePayload = {clientSubmissionId: randomUUID(), purpose: 'message', text: '生产订单接口持续502，已回滚并恢复，请根据项目流程核对应该协作的环节。'};
    fs.writeFileSync(freeFile, JSON.stringify(freePayload, null, 2));
    baseline.unlinkedSubmission = cli(['submit', '--file', freeFile]);
    const missingTask = spawnSync(process.env.JUDEX_E2E_CLI!, ['--server', baseURL!, '--project', project.id, '--no-wait',
      'report', '--kind', 'progress', '--file', freeFile],
      {encoding: 'utf8', windowsHide: true, timeout: 60000, env: {...process.env, JUDEX_TOKEN: credentials.accessToken}});
    baseline.reportWithoutTask = {exitCode: missingTask.status, stdout: missingTask.stdout, stderr: missingTask.stderr, error: missingTask.error?.message};
    if (missingTask.status === 0) baseline.failures.push({id: 'report-without-task', error: 'CLI returned success exit code for missing required task'});
    await page.goto(`/?project=${project.id}&view=team`);
    await expect(page.getByTestId('position-card-' + position.id)).toBeVisible();
    await expect(page.getByTestId('position-card-' + position.id).locator('.judex-position-nodes button')).toHaveCount(6);
    await page.screenshot({path: info.outputPath('one-position-three-workflows.png'), fullPage: true});
    fs.writeFileSync(path.join(artifact, 'platform-baseline.json'), JSON.stringify(baseline, null, 2));
    expect(baseline.failures).toEqual([]);
  } finally {
    fs.writeFileSync(path.join(artifact, 'platform-baseline.json'), JSON.stringify(baseline, null, 2));
    await context.close();
  }
});

function inferenceCases(workflow: (key: string) => any) {
  const match = (flow: string, node: string) => ({workflowId: workflow(flow).id, nodeId: node});
  const cases = [
    {id: 'feature-content', texts: ['已修复计划中的登录重复回调，本地单元测试和联调已通过，跨浏览器验证还未做。', '新迭代的登录回调去重代码完成，在开发环境跑过接口联调，还缺浏览器兼容性证据。'], expected: {decision: 'match', matches: [match('software', 'work')]}},
    {id: 'incident-content', texts: ['线上订单接口502错误率骤升至18%，回滚后恢复到0.2%，用户影响正在统计。', '生产支付服务超时影响真实用户，刚完成降级，服务恢复了，止损过程已有监控记录。'], expected: {decision: 'match', matches: [match('incident', 'work')]}},
    {id: 'baseline-content', texts: ['在隔离压测环境首次采集200并发下的RPS、P95与错误率，尚未进行代码优化。', '性能实验第一轮已跑完，固定机器与负载得到了原始延迟和吞吐数据，接下来准备尝试优化。'], expected: {decision: 'match', matches: [match('performance', 'work')]}},
    {id: 'comparison-content', texts: ['隔离压测环境固定200并发，缓存优化前P95为620ms，优化后210ms，还缺第三轮重复采样。', '同一隔离机器同一负载，对照优化前后的吞吐与延迟，已有两轮结果，错误率对照未补齐。'], expected: {decision: 'match', matches: [match('performance', 'verify')]}},
    {id: 'plan-context', task: {title: '核对实验材料', expectedOutput: '整理同负载优化前后采样证据'}, plan: {title: '缓存优化验证', goal: '在隔离压测环境对照优化前后性能'}, texts: ['两轮对照样本都齐了，第三轮尚未补充。', '前后两组数据整理好了，正在检查环境一致性。'], expected: {decision: 'match', matches: [match('performance', 'verify')]}},
    {id: 'task-context', task: {title: '核对登录功能交付证据', expectedOutput: '登录功能回归测试和联调证据清单'}, plan: {title: '新版登录交付', goal: '交付新的登录功能'}, texts: ['证据清单已整理，缺跨浏览器测试记录。', '本轮自检发现兼容性证据未补齐，需要补测。'], expected: {decision: 'match', matches: [match('software', 'verify')]}},
    {id: 'incident-root-cause', texts: ['昨天影响真实用户的订单事故已恢复，今天查明配置回源造成连接耗尽，正在核对防复发整改。', '生产故障昨天已止损，根因定位到连接池参数，整改验证和事故复盘正在进行。'], expected: {decision: 'match', matches: [match('incident', 'verify')]}},
    {id: 'ambiguous', texts: ['处理完成了，请安排下一步。', '结果已经整理好，帮我看看下一步。'], expected: {decision: 'clarify', matches: []}},
    {id: 'overlap', texts: ['接口P95超标，我已调整缓存，目前只有一轮数据。', '接口延迟很高，刚改了缓存，需要继续核对。'], expected: {decision: 'clarify', matches: []}},
    {id: 'multiple', texts: ['两项独立工作：生产订单502事故已回滚恢复；另外，本次迭代新增登录回调去重已在开发环境完成实现与联调。', '今天有两份事项：线上支付故障已降级恢复；新版登录回调去重功能也实现了，正在本地联调。'], expected: {decision: 'match', matches: [match('incident', 'work'), match('software', 'work')]}},
    {id: 'unrelated', texts: ['完成候选人面试记录与薪酬沟通，等待招聘负责人处理。', '客户宣传活动的摄影名单和场地采购单已经整理好了。'], expected: {decision: 'out_of_scope', matches: []}},
    {id: 'conflicting-task', task: {title: '登录回调去重', workflowId: workflow('software').id, nodeId: 'work'}, plan: {title: '新版登录交付', goal: '交付新的登录功能'}, texts: ['这份上报记录生产订单502事故的回滚恢复过程，与登录实现无关。', '这次提交是生产支付故障的降级止损记录，没有登录回调改动。'], expected: {decision: 'clarify', matches: []}},
  ];
  return cases;
}
