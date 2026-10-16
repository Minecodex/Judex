import {test, expect} from '@playwright/test';
import {createHash} from 'node:crypto';
import {setupCooperation, command} from './cooperation-business-helpers';
import {chooseValue} from './workspace-helpers';

test('CFG01 configuration reads every page before editing and selects concrete workflow prerequisites', async ({browser}, info) => {
  test.setTimeout(240000);
  const f = await setupCooperation(browser);
  const {a, b, ca, cb, root, project, identity, foreign, task} = f;
  try {
    const session = await command(a, 'get', '/auth/session');
    const api = async (method: string, url: string, data?: unknown) => {
      const response = await a.request.fetch('/api/v1' + url, {method, data,
        headers: {'X-CSRF-Token': session.csrfToken, 'Idempotency-Key': crypto.randomUUID()}});
      expect(response.ok(), await response.text()).toBeTruthy();
      return (await response.json()).data;
    };
    const positions: any[] = [];
    for (let i = 0; i < 103; i++) positions.push(await api('POST', root + '/positions', {name: '分页职位' + i, prompt: '配置分页验收'}));
    const first = await api('GET', root + '/positions?limit=100');
    expect(first.nextCursor).toBeTruthy();
    const later = positions.find(row => !first.items.some((head: any) => head.id === row.id));
    expect(later).toBeTruthy();
    const drafts: any[] = [];
    for (let i = 0; i < 52; i++) drafts.push(await api('POST', root + '/tasks', {title: '分页目标任务' + i}));
    const page = await api('GET', root + '/tasks?limit=50');
    const lateTask = drafts.find(row => !page.items.some((head: any) => head.id === row.id));
    expect(lateTask).toBeTruthy();
    const upload = async (text: string, sourceVersion?: string) => {
      const raw = Buffer.from(text), sha = createHash('sha256').update(raw).digest('hex');
      const upload = await api('POST', root + '/uploads', {name: '固定依据.txt', size: raw.length, sha256: sha, mime: 'text/plain', kind: 'file'});
      const part = await a.request.put('/api/v1' + root + `/uploads/${upload.id}/parts/1`, {data: raw,
        headers: {'X-CSRF-Token': session.csrfToken, 'X-Judex-Part-SHA256': sha}});
      expect(part.ok(), await part.text()).toBeTruthy();
      return api('POST', root + `/uploads/${upload.id}/complete`, {parts: [{partNumber: 1, sha256: sha}], ...(sourceVersion ? {sourceVersion} : {})});
    };
    const original = await upload('固定依据第一版'), current = await upload('固定依据第二版', original.id);
    expect(current.materialId).toBe(original.materialId);
    expect(current.revision).toBe(2);
    const started = await api('POST', root + `/tasks/${task.id}/start`, {expectedVersion: task.version, identityId: identity.id});
    await api('POST', root + `/tasks/${task.id}/reports`, {kind: 'delivery', text: '供交接引用的交付', identityId: identity.id, expectedTaskVersion: started.version});
    const handoff = await api('POST', root + '/handoffs', {title: '来源对象选择验收', kind: 'stage', targetTaskId: task.id, receiverIdentityId: foreign.id,
      sources: [{sourceTaskId: task.id, senderIdentityId: identity.id}]});
    const source = (await api('GET', root + '/handoffs')).items.find((row: any) => row.id === handoff.id).sources[0];
    const body = {name: '配置选择验收流程', nodes: [{id: 'gate', name: '核对', responsibility: '核对固定依据', allowedPositionIds: [], defaultApprovalPolicy: 'all'}], advisoryEdges: []};
    const flow = await api('POST', root + '/workflows', body);
    const cross = await api('POST', '/projects', {title: '隔离项目'});
    const foreignTask = await api('POST', `/projects/${cross.id}/tasks`, {title: '不可跨项目选择'});
    await a.goto(`/?project=${project.id}&settings=team`);
    await a.getByTestId('edit-position-' + later.id).click();
    await a.getByTestId('position-name').fill('第二页职位已修改');
    await a.getByRole('dialog').getByRole('button', {name: /保存|Save/, exact: true}).click();
    await expect(a.getByRole('dialog')).toHaveCount(0);
    const positionsAfter: any[] = [];
    let cursor = '';
    do {const result = await api('GET', root + '/positions?limit=100' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : '')); positionsAfter.push(...result.items); cursor = result.nextCursor ?? '';} while (cursor);
    expect(positionsAfter.find(row => row.id === later.id).name).toBe('第二页职位已修改');
    await a.goto(`/?project=${project.id}&settings=flows&view=flows&item=${flow.id}`);
    await expect(a.getByTestId('workflow-draft-editor')).toBeVisible();
    await a.getByTestId('workflow-add-hard-rule').click();
    await chooseValue(a, 'workflow-hard-rule-kind-0', 'task_acceptance');
    const target = a.getByTestId('workflow-hard-rule-target-0');
    const picker = a.locator('.judex-workflow-target-picker');
    await expect(picker.getByRole('button', {name: /加载更多|Load more/})).toBeVisible();
    await picker.getByRole('button', {name: /加载更多|Load more/}).click();
    await chooseValue(a, 'workflow-hard-rule-target-0', lateTask.id);
    await a.getByTestId('workflow-hard-rule-target-0-search').fill('不存在的搜索');
    await expect(target).toContainText(lateTask.title);
    await a.getByTestId('workflow-hard-rule-target-0-search').fill('');
    await target.click();
    await expect(a.locator(`[data-option-value="${foreignTask.id}"]`)).toHaveCount(0);
    await a.keyboard.press('Escape');
    await a.getByRole('listbox', {includeHidden: true}).waitFor({state: 'detached'});
    await chooseValue(a, 'workflow-hard-rule-kind-0', 'handoff_receipt');
    await expect(target).toHaveAttribute('data-value', '');
    await chooseValue(a, 'workflow-hard-rule-target-0', source.id);
    await expect(target).toContainText('来源对象选择验收');
    await chooseValue(a, 'workflow-hard-rule-kind-0', 'material_ready');
    await expect(target).toHaveAttribute('data-value', '');
    await chooseValue(a, 'workflow-hard-rule-target-0', current.id);
    await chooseValue(a, 'workflow-hard-rule-target-0-version', original.id);
    await expect(target).toContainText('v1');
    await a.getByTestId('save-workflow-draft').click();
    await expect(a.getByTestId('publish-workflow-draft')).toBeEnabled();
    const versions = await api('GET', root + `/workflows/${flow.id}/versions`);
    expect(versions.items.find((row: any) => row.state === 'draft').body.hardRules[0].targetId).toBe(original.id);
    await a.reload();
    await expect(a.getByTestId('workflow-hard-rule-target-0')).toContainText('v1');
    await a.screenshot({path: info.outputPath('fixed-workflow-target.png'), fullPage: true});
    expect((await command(b, 'get', root + `/tasks/${task.id}`)).status).toBe('delivered');
    await info.attach('configuration-proof', {body: JSON.stringify({project: project.id, secondPagePosition: later.id, secondPageTask: lateTask.id, fixedVersion: original.id, currentVersion: current.id, source: source.id}), contentType: 'application/json'});
  } finally {await ca.close(); await cb.close();}
});
