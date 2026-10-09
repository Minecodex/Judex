import {test, expect} from '@playwright/test';
import {randomUUID} from 'node:crypto';
import {command, setupCooperation} from './cooperation-business-helpers';
import {uploadMaterial, materialWrite, stableMaterialDialog, chooseMaterialContext} from './materials-business-helpers';
test.setTimeout(240000);

test('MAT05 object navigation, resolved plan and sharing preserve the actual discussion draft', async ({browser}, info) => {
  const f = await setupCooperation(browser), {a, ca, cb, root, project, task, plan} = f;
  try {
    const file = await uploadMaterial(a, root, 'acceptance.md');
    await command(a, 'post', root + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'material', text: '核对任务材料', taskId: task.id, materialVersionIds: [file.id]});
    const main = (await command(a, 'post', root + '/tasks/' + task.id + '/main-topic', {})).topic;
    await a.goto('/projects/' + project.id + '?tab=materials');
    await a.locator('.judex-material-card').getByRole('button', {name: '详情', exact: true}).click();await stableMaterialDialog(a);
    await a.getByRole('dialog').getByRole('button', {name: task.title, exact: true}).click();
    await expect(a.getByTestId('task-details-drawer')).toBeVisible();await expect(a.getByTestId('task-inspector')).toContainText(task.title);
    for (const [scope, topic] of [['plans/' + plan.id, plan.mainTopicId], ['tasks/' + task.id, main.id]]) {
      await a.goto('/projects/' + project.id + '/' + scope + '/chat/' + topic + '?view=resources');
      await a.getByTestId('work-discussion-input').fill('保留当前讨论草稿');
      await a.locator('.judex-material-library-panel .judex-material-card').getByRole('button', {name: '详情', exact: true}).click();
      await a.getByRole('dialog').getByRole('button', {name: task.title, exact: true}).click();
      await expect(a.getByTestId('task-inspector')).toContainText(task.title);await expect(a.getByTestId('work-discussion-input')).toHaveValue('保留当前讨论草稿');
    }
    await a.goto('/projects/' + project.id + '/tasks/' + task.id + '/chat/' + main.id + '?view=resources');
    await a.locator('.judex-material-library-panel').getByRole('button', {name: '上传资料', exact: true}).click();await stableMaterialDialog(a);
    await expect(a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(0)).toHaveAttribute('data-value', plan.id);
    await expect(a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(1)).toHaveAttribute('data-value', task.id);
    await a.screenshot({path: info.outputPath('resolved-task-plan.png')});await a.getByRole('dialog').getByRole('button', {name: '取消', exact: true}).click();
    await a.getByTestId('work-discussion-input').fill('分享之前仍保留的草稿');
    const before = (await command(a, 'get', root + '/material-versions/' + file.id + '/usages')).items.length;
    const topicsBefore = (await command(a, 'get', root + '/topics')).totalCount;
    await a.locator('.judex-material-library-panel .judex-material-card').getByRole('button', {name: '详情', exact: true}).click();
    await a.getByRole('dialog').getByRole('button', {name: '发送到讨论', exact: true}).click();await stableMaterialDialog(a);
    await expect(a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(0)).toHaveAttribute('data-value', main.id);
    await expect(a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(1)).toHaveAttribute('data-value', 'task:' + task.id);
    await a.getByRole('dialog').getByRole('button', {name: '添加到消息', exact: true}).click();
    await expect(a.getByTestId('work-discussion-input')).toHaveValue('分享之前仍保留的草稿');await expect(a.locator('.judex-work-upload')).toContainText('acceptance.md');
    expect((await command(a, 'get', root + '/material-versions/' + file.id + '/usages')).items.length).toBe(before);
    await a.getByTestId('send-work-message').click();
    await expect.poll(async () => (await command(a, 'get', root + '/material-versions/' + file.id + '/usages')).items.filter((u: any) => u.kind === 'message').length).toBe(1);
    expect((await command(a, 'get', root + '/materials')).totalCount).toBe(1);expect((await command(a, 'get', root + '/topics')).totalCount).toBe(topicsBefore);
    expect((await command(a, 'get', root + '/tasks/' + task.id)).status).toBe('ready');
    await a.goto('/projects/' + project.id + '?tab=materials');await a.locator('.judex-material-card').getByRole('button', {name: '详情', exact: true}).click();
    await a.getByRole('dialog').getByRole('button', {name: '发送到讨论', exact: true}).click();await stableMaterialDialog(a);
    await chooseMaterialContext(a, 0, main.id);await chooseMaterialContext(a, 1, 'task:' + task.id);
    await a.getByRole('dialog').getByRole('button', {name: '添加到消息', exact: true}).click();await expect(a.locator('.judex-work-upload')).toContainText('acceptance.md');
    await a.screenshot({path: info.outputPath('library-share-draft.png')});
  } finally {await ca.close();await cb.close();}
});

test('MAT06 uploader and manager deletion, paged uses, cross-project rejection and inherited fixed versions', async ({browser}, info) => {
  const f = await setupCooperation(browser), {a, b, ca, cb, root, project, task} = f;
  try {
    const file = await uploadMaterial(b, root, 'acceptance.md'), main = (await command(a, 'post', root + '/tasks/' + task.id + '/main-topic', {})).topic;
    for (let n = 0; n < 12; n++) await command(a, 'post', root + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'material', text: '用途说明 ' + n, taskId: task.id, materialVersionIds: [file.id]});
    await command(a, 'post', root + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'message', text: '原始材料使用事实', topicId: main.id, taskId: task.id, materialVersionIds: [file.id]});
    const seen = new Set<string>();let cursor = '';
    do {const page = await command(a, 'get', root + '/material-versions/' + file.id + '/usages?limit=3' + (cursor ? '&cursor=' + encodeURIComponent(cursor) : ''));expect(page.items.length).toBeLessThanOrEqual(3);for (const u of page.items) {expect(seen.has(u.id)).toBe(false);seen.add(u.id);}cursor = page.nextCursor ?? '';} while (cursor);
    expect(seen.size).toBe(13);
    const fork = await command(a, 'post', root + '/topics/' + main.id + '/fork', {title: '共享历史资料复核', forkAfterSeq: 1});
    const forkId = fork.topic?.id ?? fork.id;
    const history = (await command(a, 'get', root + '/topics/' + forkId + '/messages')).items;
    expect(history.some((m: any) => m.inherited && m.materials.some((v: any) => v.versionId === file.id))).toBe(true);
    expect((await command(a, 'get', root + '/material-versions/' + file.id + '/usages?limit=100')).items.length).toBe(13);
    const foreign = await command(a, 'post', '/projects', {title: '外部材料项目'}), foreignRoot = '/projects/' + foreign.id;
    const otherPlan = await command(a, 'post', foreignRoot + '/plans', {title: '其他计划'});
    expect((await materialWrite(a, 'POST', root + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'material', text: '跨计划必须拒绝', planId: otherPlan.id, materialVersionIds: [file.id]})).status()).toBe(400);
    expect((await materialWrite(a, 'POST', foreignRoot + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'material', text: '跨材料必须拒绝', materialVersionIds: [file.id]})).status()).toBe(400);
    expect((await materialWrite(b, 'DELETE', root + '/materials/' + file.materialId, {expectedCurrentVersionId: randomUUID()})).status()).toBe(409);
    await a.goto('/projects/' + project.id + '?tab=materials');await a.locator('.judex-material-card').getByRole('button', {name: '详情', exact: true}).click();
    await expect(a.getByRole('dialog').locator('.judex-material-timeline article')).toHaveCount(11);await a.getByRole('dialog').getByRole('button', {name: '加载更多', exact: true}).click();
    await expect(a.getByRole('dialog').locator('.judex-material-timeline article')).toHaveCount(14);await a.getByRole('dialog').getByRole('button', {name: '关闭', exact: true}).last().click();
    expect((await materialWrite(b, 'DELETE', root + '/materials/' + file.materialId, {expectedCurrentVersionId: file.id})).status()).toBe(200);
    await a.goto('/projects/' + project.id + '/tasks/' + task.id + '/chat/' + main.id);await expect(a.locator('.judex-material-deleted')).toContainText('保留历史依据');
    expect((await a.request.get('/api/v1' + root + '/material-versions/' + file.id + '/content')).ok()).toBe(true);
    const own = await uploadMaterial(a, root, 'acceptance.json'), memberId = (await command(b, 'get', '/auth/session')).user.id;
    const projectVersion = (await command(a, 'get', root)).version;
    const promoted = await materialWrite(a, 'PATCH', root + '/members/' + memberId, {role: 'manager', expectedVersion: projectVersion});expect(promoted.ok(), await promoted.text()).toBe(true);
    expect((await materialWrite(b, 'DELETE', root + '/materials/' + own.materialId, {expectedCurrentVersionId: own.id})).status()).toBe(200);
    await a.screenshot({path: info.outputPath('deleted-fixed-reference.png')});
  } finally {await ca.close();await cb.close();}
});

test('MAT07 eight real formats share covers and fit every desktop discussion entrance', async ({browser}, info) => {
  const f = await setupCooperation(browser), {a, ca, cb, root, project, task, plan} = f;const errors: string[] = [];a.on('pageerror', e => errors.push(e.message));
  try {
    const versions: string[] = [];
    for (const ext of ['pdf', 'docx', 'xlsx', 'png', 'md', 'pptx', 'json', 'zip']) {
      const file = await uploadMaterial(a, root, 'acceptance.' + ext);versions.push(file.id);
      await command(a, 'post', root + '/material-versions/' + file.id + '/preview');
      await expect.poll(async () => (await command(a, 'get', root + '/material-versions/' + file.id + '/preview')).status, {timeout: 120000}).toBe('ready');
      const preview = await command(a, 'get', root + '/material-versions/' + file.id + '/preview');expect(preview.contentUrl).toContain(file.id);expect(preview.thumbnail.url).toBe(preview.contentUrl);if (preview.kind === 'pdf') expect(preview.thumbnail.page).toBe(1);
    }
    await command(a, 'post', root + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'material', text: '统一真实文件范围', taskId: task.id, materialVersionIds: versions});
    const main = (await command(a, 'post', root + '/tasks/' + task.id + '/main-topic', {})).topic;
    for (const [id, context] of [[plan.mainTopicId, {planId: plan.id}], [main.id, {taskId: task.id}]] as const) await command(a, 'post', root + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'message', text: '八类固定资料用于讨论', topicId: id, ...context, materialVersionIds: versions});
    const entries = {library: '/projects/' + project.id + '?tab=materials', plan: '/projects/' + project.id + '/plans/' + plan.id + '/chat/' + plan.mainTopicId + '?view=resources', task: '/projects/' + project.id + '/tasks/' + task.id + '/chat/' + main.id + '?view=resources'};
    for (const width of [1120, 1440, 1920, 3840]) for (const locale of ['zh-CN', 'en']) for (const theme of ['light', 'dark']) for (const [entry, url] of Object.entries(entries)) {
      await a.setViewportSize({width, height: 1000});await a.evaluate(({locale, theme}) => {localStorage.setItem('argus.locale', locale);localStorage.setItem('judex.theme', theme);}, {locale, theme});await a.goto(url);
      await expect(a.locator('.judex-material-grid>.judex-material-card')).toHaveCount(8);
      if (entry !== 'library') await expect(a.locator('.judex-material-message-grid>.judex-material-card')).toHaveCount(8);
      const metrics = await a.evaluate(() => ({root: document.documentElement.scrollWidth <= innerWidth, panes: [...document.querySelectorAll<HTMLElement>('.judex-chat-thread,.judex-chat-panel-body')].every(e => e.scrollWidth <= e.clientWidth + 1)}));expect(metrics).toEqual({root: true, panes: true});
      const grid=await a.locator('.judex-material-grid').evaluate((e:HTMLElement)=>({columns:getComputedStyle(e).gridTemplateColumns.split(' ').length,available:e.parentElement!.clientWidth-parseFloat(getComputedStyle(e.parentElement!).paddingLeft)-parseFloat(getComputedStyle(e.parentElement!).paddingRight)}));
      expect(grid.columns).toBe(entry==='library'?4:grid.available<=380?1:2);
      if (width === 1440 && locale === 'zh-CN' && theme === 'light') await a.screenshot({path: info.outputPath(entry + '-eight-formats.png')});
    }
    await a.evaluate(() => {localStorage.setItem('argus.locale', 'zh-CN');localStorage.setItem('judex.theme', 'light');});await a.setViewportSize({width: 1440, height: 1400});await a.goto(entries.library);
    await expect(a.locator('.judex-material-code-art')).toHaveCount(2);await expect(a.locator('.judex-material-folder-art')).toContainText('2 个文件');
    const pdfCanvases=a.locator('.judex-material-pdf-thumbnail canvas');await expect(pdfCanvases).toHaveCount(4);
    await expect.poll(()=>pdfCanvases.evaluateAll((els:any[])=>els.every(canvas=>{const pixels=canvas.getContext('2d').getImageData(0,0,canvas.width,canvas.height).data;for(let i=0;i<pixels.length;i+=4)if(pixels[i+3]&&pixels[i]<200)return true;return false;})),{timeout:30000}).toBe(true);
    await a.screenshot({path: info.outputPath('complete-library.png')});
    await a.goto(entries.task);await a.getByRole('button', {name: '添加资料', exact: true}).click();await a.getByRole('button', {name: '选择共享资料', exact: true}).click();await stableMaterialDialog(a);
    await expect(a.getByRole('dialog').locator('.judex-material-thumbnail')).toHaveCount(8);await expect(a.getByRole('dialog').locator('.judex-material-code-art')).toHaveCount(2);await a.screenshot({path: info.outputPath('complete-picker.png')});
    expect(errors).toEqual([]);
  } finally {await ca.close();await cb.close();}
});

test('MAT08 safe Markdown renders emphasis and tables while file HTML cannot execute', async ({browser}, info) => {
  const f = await setupCooperation(browser), {a, ca, cb, root, project} = f;
  try {
    const raw = Buffer.from('# 真正的 Markdown\n\n**用途强调**、*说明*和 [安全链接](https://example.org)\n\n| 项目 | 说明 |\n| --- | --- |\n| 文件 | 固定版本 |\n\n<script>window.materialAttack=1</script>\n\n![禁止外部图片](https://example.org/material-xss-probe.png)\n\n[不可执行链接](javascript:alert(1))\n');
    const file = await uploadMaterial(a, root, '格式与安全.md', raw);await a.goto('/projects/' + project.id + '?tab=materials');await a.locator('.judex-material-cover').click();
    await expect(a.getByRole('dialog').locator('.judex-material-markdown strong')).toHaveText('用途强调');await expect(a.getByRole('dialog').locator('.judex-material-markdown table')).toContainText('固定版本');
    expect(await a.evaluate(() => (window as any).materialAttack)).toBeUndefined();await expect(a.locator('.judex-material-markdown img,.judex-material-markdown script,.judex-material-markdown a[href^="javascript:"]')).toHaveCount(0);
    expect((await a.request.get('/api/v1' + root + '/material-versions/' + file.id + '/content')).ok()).toBe(true);await a.screenshot({path: info.outputPath('safe-markdown.png')});
    await a.getByRole('button',{name:'关闭',exact:true}).last().click();
    const chinese = await uploadMaterial(a, root, 'chinese-cid.pdf');
    await command(a,'post',root+'/material-versions/'+chinese.id+'/preview');
    await expect.poll(async()=>(await command(a,'get',root+'/material-versions/'+chinese.id+'/preview')).status).toBe('ready');
    await a.goto('/projects/'+project.id+'?tab=materials');await a.locator('.judex-material-card').filter({hasText:'chinese-cid.pdf'}).locator('.judex-material-cover').click();
    await expect(a.getByRole('dialog').locator('.judex-material-reader-toolbar')).toContainText('1 / 1');
    await expect(a.getByRole('dialog').locator('canvas')).toBeVisible();
    const cmap=await a.request.get('/assets/pdf-support/6.4.299/cmaps/Adobe-GB1-UCS2.bcmap');expect(cmap.ok()).toBe(true);expect((await cmap.body()).length).toBeGreaterThan(100);
    await a.screenshot({path:info.outputPath('chinese-cid-preview.png')});
  } finally {await ca.close();await cb.close();}
});

test('MAT09 current objects outside the first page remain selected and library counts cover all pages',async({browser},info)=>{
  const f=await setupCooperation(browser),{a,ca,cb,root,project,task,plan,identity,flow}=f;
  try{
    for(let n=0;n<51;n++)await command(a,'post',root+'/tasks',{title:'分页任务 '+n,expectedOutput:'范围选择验证',acceptanceCriteria:'对象不遗漏',planId:plan.id,participantIdentityIds:[identity.id],reviewerIdentityId:identity.id,workflowId:flow.id,nodeId:'work'});
    const main=(await command(a,'post',root+'/tasks/'+task.id+'/main-topic',{})).topic;
    await a.goto('/projects/'+project.id+'/tasks/'+task.id+'/chat/'+main.id+'?view=resources');
    await a.locator('.judex-material-library-panel').getByRole('button',{name:'上传资料',exact:true}).click();await stableMaterialDialog(a);
    await expect(a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(0)).toHaveAttribute('data-value',plan.id);
    await expect(a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(1)).toContainText(task.title);
    await a.getByRole('dialog').getByRole('button',{name:'加载更多 · 关联任务',exact:true}).click();
    await a.getByRole('dialog').locator('.judex-ui-select [data-value]').nth(1).click();await expect(a.getByRole('option')).toHaveCount(53);
    await a.keyboard.press('Escape');await a.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();
    for(let n=0;n<21;n++)await uploadMaterial(a,root,'分页文件'+n+'.txt',Buffer.from('不可变原件 '+n));
    await a.goto('/projects/'+project.id+'?tab=materials');await expect(a.getByTestId('material-count')).toContainText('21 份资料');await expect(a.locator('.judex-material-grid>.judex-material-card')).toHaveCount(20);
    await a.getByRole('button',{name:'加载更多',exact:true}).click();await expect(a.locator('.judex-material-grid>.judex-material-card')).toHaveCount(21);await expect(a.getByTestId('material-count')).toHaveText('21 份资料');
    await a.getByRole('textbox',{name:'搜索文件、上传人或用途…',exact:true}).fill('分页文件20');await expect(a.getByTestId('material-count')).toHaveText('1 份资料');
    expect((await a.request.get('/api/v1'+root+'/materials?formatGroup=invalid')).status()).toBe(400);
    await a.screenshot({path:info.outputPath('material-filter-count.png')});
  }finally{await ca.close();await cb.close();}
});
