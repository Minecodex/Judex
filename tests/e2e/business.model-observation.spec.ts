import {expect, test} from '@playwright/test';
import {execFileSync, spawnSync} from 'node:child_process';
import path from 'node:path';
import {command, registerMember} from './cooperation-business-helpers';
import {fixtureSQL} from './fixture-database';

async function cacheGatewayMode(mode: string) {
  if (process.env.JUDEX_E2E_GATEWAY_ADMIN_URL) {
    const admin = new URL(process.env.JUDEX_E2E_GATEWAY_ADMIN_URL);
    expect(admin.hostname).toBe('127.0.0.1');
    expect(process.env.JUDEX_E2E_RUN_ID).toMatch(/^judex-e2e-\d+-\d+$/);
    const response = await fetch(new URL('/mode/'+mode, admin), {method:'POST'});
    expect(response.status).toBe(204);
    return;
  }
  const namespace = path.basename(process.env.JUDEX_E2E_ARTIFACT ?? '');
  expect(namespace).toMatch(/^judex-ui-[a-f0-9]{8}$/);
  const state = JSON.parse(execFileSync('kubectl', ['get', 'namespace', namespace, '-o', 'json'], {encoding:'utf8', windowsHide:true}));
  expect(state.metadata.labels['judex.dev/test-run']).toBe(namespace);
  execFileSync('kubectl', ['-n', namespace, 'exec', 'deployment/controlled-gateway', '--', 'wget', '-q', '-O', '-', '--post-data=', 'http://127.0.0.1:8080/mode/'+mode], {windowsHide:true});
}

test('CACHE01 real browser and worker retain cache usage for 24 hours without deleting call accounting', async ({page}, info) => {
  test.setTimeout(120000);
  await cacheGatewayMode('cache');
  try {
    await registerMember(page, 'cache-observer');
    const project = await command(page, 'post', '/projects', {title:'缓存观测隔离验收'});
    const root = '/projects/'+project.id;
    const topic = await command(page, 'post', root+'/topics', {title:'缓存观测', initialMessage:'验证真实平台调用、用量落库和自动过期。'});
    const messages = (await command(page, 'get', root+'/topics/'+topic.id+'/messages')).items;
    const source = messages.find((message:any) => message.sourceSubmissionId || message.submissionId);
    expect(source).toBeTruthy();
    const run = await command(page, 'post', root+'/topics/'+topic.id+'/runs', {sourceSubmissionId:source.sourceSubmissionId ?? source.submissionId});
    await expect.poll(async () => (await command(page, 'get', root+'/runs/'+run.runId)).state, {timeout:30000}).toBe('succeeded');
    await page.goto('/projects/'+project.id);
    await page.getByRole('button', {name:'项目讨论记录',exact:true}).click();
    await page.getByTestId('scoped-topic-'+topic.id).click();
    await expect(page.getByTestId('chat-thread')).toContainText('缓存观测协议验收完成');
    const snapshot = JSON.parse(fixtureSQL(`SELECT jsonb_build_object('id',id,'attempt',attempt,'usage',usage_json,'known',usage_known,'providerId',provider_request_id) FROM model_calls WHERE run_id='${run.runId}' ORDER BY attempt DESC LIMIT 1`));
    expect(snapshot.known).toBe(true);
    expect(snapshot.usage.totalTokens).toBe(1050);
    expect(snapshot.usage.observation).toMatchObject({modelName:'cache-fixture',inputTokens:1000,outputTokens:50,cacheReadTokens:800,cacheWriteTokens:null});
    expect(snapshot.providerId).toMatch(/^cache-fixture-\d+$/);
    const cli = process.env.JUDEX_E2E_CLI!;
    expect(cli).toBeTruthy();
    const failed = spawnSync(cli, ['report', '--json'], {encoding:'utf8', windowsHide:true, env:{...process.env,JUDEX_CONFIG_DIR:info.outputPath('cli-profile'),JUDEX_TOKEN:''}});
    expect(failed.status).toBe(2);
    expect(failed.stderr).toBe('');
    expect(JSON.parse(failed.stdout)).toMatchObject({schemaVersion:'1',ok:false});
    fixtureSQL(`UPDATE model_calls SET ended_at=now()-interval '25 hours' WHERE id='${snapshot.id}'`);
    expect(fixtureSQL(`SELECT count(*) FROM model_calls WHERE id='${snapshot.id}' AND ended_at>now()-interval '24 hours' AND usage_json ? 'observation'`).trim()).toBe('0');
    await expect.poll(() => fixtureSQL(`SELECT usage_json ? 'observation' FROM model_calls WHERE id='${snapshot.id}'`).trim(), {timeout:75000,intervals:[500,1000,2000]}).toBe('f');
    const expired = JSON.parse(fixtureSQL(`SELECT jsonb_build_object('attempt',attempt,'usage',usage_json,'known',usage_known,'providerId',provider_request_id) FROM model_calls WHERE id='${snapshot.id}'`));
    expect(expired).toMatchObject({attempt:snapshot.attempt,known:true,providerId:null});
    expect(expired.usage).toEqual({totalTokens:1050,conservative:false});
    expect(Number(fixtureSQL(`SELECT used_tokens FROM discussion_batches WHERE id='${run.batchId}'`).trim())).toBeGreaterThanOrEqual(1050);
    await info.attach('cache-observation-proof', {body:JSON.stringify({source:'controlled gateway; real browser, worker, PG and CLI',snapshot,expired}),contentType:'application/json'});
  } finally {
    await cacheGatewayMode('collaboration');
  }
});
