// Recheck selected failed live samples without creating another deployment.
import {execFileSync, spawn} from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
const [fixtureFile, cases = 'overlap,conflicting-task', outputTokens = '8192'] = process.argv.slice(2);
if (!fixtureFile || !fs.existsSync(fixtureFile)) throw new Error('Pass the saved owned experiment fixture');
if (!/^\d+$/.test(outputTokens) || Number(outputTokens) < 4096 || Number(outputTokens) > 16384) throw new Error('Invalid explicit output budget');
const namespace = process.env.JUDEX_ROUTING_SOURCE_NAMESPACE ?? 'judex';
const deployment = process.env.JUDEX_ROUTING_SOURCE_DEPLOYMENT ?? 'judex';
const context = process.env.JUDEX_ROUTING_CONTEXT ?? 'docker-desktop';
const kube = (...args) => execFileSync('kubectl', ['--context', context, ...args], {encoding: 'utf8', windowsHide: true, timeout: 30000});
const container = JSON.parse(kube('-n', namespace, 'get', 'deployment', deployment, '-o', 'json')).spec.template.spec.containers.find(item => item.name === 'server');
const resolve = name => {
  const value = container.env?.find(item => item.name === name);
  if (value?.value !== undefined) return value.value;
  if (value?.valueFrom?.secretKeyRef) {
    const ref = value.valueFrom.secretKeyRef;
    return Buffer.from(JSON.parse(kube('-n', namespace, 'get', 'secret', ref.name, '-o', 'json')).data[ref.key], 'base64').toString();
  }
  for (const ref of container.envFrom ?? []) if (ref.secretRef) {
    const data = JSON.parse(kube('-n', namespace, 'get', 'secret', ref.secretRef.name, '-o', 'json')).data;
    if (data[name]) return Buffer.from(data[name], 'base64').toString();
  }
  throw new Error('Missing model setting ' + name);
};
const results = path.join(path.dirname(path.resolve(fixtureFile)), 'model-routing-recheck-' + outputTokens + '.json');
if (fs.existsSync(results)) throw new Error('Recheck evidence already exists; preserve it before another run');
const command = spawn('go', ['test', './tests/agent/live', '-run', '^TestLiveWorkflowRouting$', '-count=1', '-v', '-timeout', '15m'],
  {windowsHide: true, stdio: 'inherit', env: {...process.env, JUDEX_ROUTING_FIXTURE: path.resolve(fixtureFile), JUDEX_ROUTING_CASE_IDS: cases,
    JUDEX_ROUTING_RESULTS: results, JUDEX_ROUTING_MAX_OUTPUT_TOKENS: outputTokens,
    JUDEX_LIVE_PROTOCOL: resolve('JUDEX_MODEL_PROTOCOL'), JUDEX_LIVE_BASE_URL: resolve('JUDEX_MODEL_BASE_URL'),
    JUDEX_LIVE_MODEL: resolve('JUDEX_MODEL_NAME'), JUDEX_LIVE_API_KEY: resolve('JUDEX_MODEL_API_KEY')}});
const status = await new Promise((resolve, reject) => {command.once('error', reject); command.once('exit', resolve);});
console.log('Targeted real-model recheck evidence: ' + results); process.exitCode = status ?? 1;
