import {createHash, randomUUID} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

// Optional real-model acceptance. Copy only the referenced model key into the
// owned namespace; credentials and private model contexts are never exported.
export async function liveAcceptance({sourceNamespace, kube, apply, ownership, root, release, main, previewPort, forward, forwards, artifact, results, until}) {
  const original = JSON.parse(kube('-n', sourceNamespace, 'get', 'deployment', process.env.JUDEX_LIVE_SOURCE_DEPLOYMENT ?? 'judex', '-o', 'json'));
  const environment = original.spec.template.spec.containers[0].env;
  const value = name => environment.find(item => item.name === name)?.value;
  const reference = environment.find(item => item.name === 'JUDEX_MODEL_API_KEY')?.valueFrom?.secretKeyRef;
  if (!reference || !value('JUDEX_MODEL_PROTOCOL') || !value('JUDEX_MODEL_BASE_URL') || !value('JUDEX_MODEL_NAME')) throw new Error('Source deployment lacks an explicit model gateway/Secret reference');
  const source = JSON.parse(kube('-n', sourceNamespace, 'get', 'secret', reference.name, '-o', 'json'));
  if (!source.data?.[reference.key]) throw new Error('Referenced model key missing');
  ownership(root);
  apply({apiVersion: 'v1', kind: 'Secret', metadata: {namespace: root, name: 'live-model-auth'}, type: 'Opaque', data: {'api-key': source.data[reference.key]}});
  const target = JSON.parse(kube('-n', root, 'get', 'deployment', release, '-o', 'json'));
  const names = ['JUDEX_MODEL_PROTOCOL', 'JUDEX_MODEL_BASE_URL', 'JUDEX_MODEL_NAME', 'JUDEX_MODEL_API_KEY'];
  const updated = target.spec.template.spec.containers[0].env.filter(item => !names.includes(item.name));
  for (const name of names.slice(0, 3)) updated.push({name, value: value(name)});
  updated.push({name: 'JUDEX_MODEL_API_KEY', valueFrom: {secretKeyRef: {name: 'live-model-auth', key: 'api-key'}}});
  kube('-n', root, 'patch', 'deployment', release, '--type=json', '-p', JSON.stringify([{op: 'replace', path: '/spec/template/spec/containers/0/env', value: updated}]));
  kube('-n', root, 'rollout', 'status', 'deployment/' + release, '--timeout=150s'); await main.reconnect();
  for (const child of forwards) if (child.spawnargs.includes(`${previewPort}:8080`)) { child.kill(); forwards.delete(child); }
  await forward(root, release, previewPort);
  const user = (await main.api.json('/auth/session')).user;
  const samples = [
    {name: '研发', text: `# 研发验收样本
目标：给项目任务增加可重试的成果上报。成员在本地执行工作，平台只记录材料与报告。
约束：同一提交键重试只能形成一份报告；不同内容复用旧键必须拒绝。来源版本过期不得覆盖当前成果。任务验收由当前审核职责持有人明确决定，模型不能验收。
已知证据：样本包含设计说明，还没有真实并发测试记录。
异议：质量岗位反对在缺少并发测试和断网重试证据时宣布完成。
待确认：并发样本规模、网络故障覆盖范围、验收人安排。请保留这些缺口。`},
    {name: '设计', text: `# 设计验收样本
目标：设计一个桌面端项目成果审阅页。左侧是材料版本，中间是固定内容，右侧是职责、意见和人工决定。
约束：统一组件、字体和间距，支持深浅色以及中英文。展示本次审阅的固定版本和来源，正文变化后不能套用旧决定。
已知证据：只有文字交互说明，没有用户可用性测试。
异议：设计岗位反对只显示审批按钮却不展示成果正文，也反对把动画或模型成功当作验收完成。
待确认：长文本阅读、对比查看以及大字体使用方式。请保留这些缺口。`},
  ];
  for (const sample of samples) {
    console.log('Real model scenario', sample.name);
    const project = await main.api.json('/projects', {title: '真实中文' + sample.name + '样本'}), prefix = `/projects/${project.id}`;
    const identities = [];
    for (const name of ['分析岗位', '质量岗位']) { const position = await main.api.json(prefix + '/positions', {name, prompt: `你负责${name}。用 read_material 读取原件，给出不超过150字的中文意见，保留异议与缺证。不发布文件、不创建提案、不委派其他岗位。`}); identities.push(await main.api.json(prefix + '/identities', {positionId: position.id, userId: user.id})); }
    const bytes = Buffer.from(sample.text), sha = createHash('sha256').update(bytes).digest('hex');
    const upload = await main.api.json(prefix + '/uploads', {name: sample.name + '说明.md', kind: 'file', size: bytes.length, sha256: sha, mime: 'text/markdown'});
    await main.api.json(`${prefix}/uploads/${upload.id}/parts/1`, bytes, 'PUT', {'Content-Type': 'application/octet-stream', 'X-Judex-Part-SHA256': sha});
    const material = await main.api.json(`${prefix}/uploads/${upload.id}/complete`, {}), topic = (await main.api.json(prefix + '/topics')).items[0];
    const submission = await main.api.json(prefix + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'message', topicId: topic.id, materialVersionIds: [material.id], text: `这是平台分析产物链路验收。先 read_material 读取固定版本 ${material.id}。必须分别 call_agent ${identities[0].id} 和 ${identities[1].id} 各一次，每岗150字以内。取得结果后你在 /workspace/review.html 写一个不超过3KB的中文HTML，显示两岗意见、异议和待人工确认事项，不得自动验收或创建提案。HTML 标题是“Judex ${sample.name}样本评审”，包含脚本 document.documentElement.dataset.artifactReady='yes'。用 publish 登记该文件：kind=html_bundle，entrypoint=index.html，sourceVersionIds=[${material.id}]，idempotencyKey=${sample.name}-report。最后简短说明材料ID和未决项。`});
    const started = await main.api.json(`${prefix}/topics/${topic.id}/runs`, {sourceSubmissionId: submission.id}); let outcome;
    await until(async () => { outcome = await main.api.json(`${prefix}/runs/${started.runId}`); return ['succeeded', 'failed', 'waiting_human', 'cancelled'].includes(outcome.state); }, 'real model ' + sample.name, 600000);
    fs.writeFileSync(path.join(artifact, 'live-' + sample.name + '-result.json'), JSON.stringify(outcome, null, 2));
    if (outcome.state !== 'succeeded' || !outcome.artifactRefs.length) throw new Error('Real scenario did not publish: ' + sample.name + ' ' + outcome.state);
    const proof = JSON.parse(kube('-n', root, 'exec', release + '-postgresql-0', '--', 'psql', '-U', 'judex', '-d', 'judex', '-Atc', `SELECT json_build_object('successfulChildren',(SELECT count(*) FROM agent_runs WHERE parent_run_id='${started.runId}' AND state='succeeded'),'modelCalls',(SELECT count(*) FROM model_calls WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id='${started.batchId}')),'tokens',(SELECT used_tokens FROM discussion_batches WHERE id='${started.batchId}'),'publishedVersions',(SELECT count(*) FROM material_versions WHERE run_id IN(SELECT id FROM agent_runs WHERE batch_id='${started.batchId}')));`));
    if (proof.successfulChildren < 2) throw new Error('Missing two real position results');
    const ref = outcome.artifactRefs[0], preview = await main.api.json(`${prefix}/materials/${ref.materialId}/versions/${ref.versionId}/preview-session`, {});
    const {chromium} = await import('@playwright/test'); const browser = await chromium.launch({channel: process.env.PLAYWRIGHT_CHANNEL ?? 'msedge', headless: true});
    try { const page = await browser.newPage(); await page.goto(preview.previewUrl); await page.getByRole('heading', {name: 'Judex ' + sample.name + '样本评审', exact: true}).waitFor(); await until(async () => (await page.locator('html').getAttribute('data-artifact-ready')) === 'yes', 'live HTML script', 10000); const text = await page.locator('body').innerText(); if (!text.includes('异议') || !text.includes('确认')) throw new Error('Published page omitted unresolved decision context'); await page.screenshot({path: path.join(artifact, 'live-' + sample.name + '.png'), fullPage: true}); } finally { await browser.close(); }
    if ((await main.api.json(prefix + '/tasks')).items.length || (await main.api.json(prefix + '/proposals')).items.length) throw new Error('Analysis created unrequested business work');
    results.push({scenario: 'real-model-' + sample.name, model: value('JUDEX_MODEL_NAME'), projectId: project.id, sourceVersionId: material.id, runId: started.runId, batchId: started.batchId, artifact: ref, proof, browserRendered: true, businessWorkUnchanged: true});
    console.log('Real model scenario passed', sample.name);
  }
}
