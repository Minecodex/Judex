// Real-model experiment against an owned copy of the running Judex image.
// Existing deployments/data are read only. Credentials never enter artifacts.
import {execFileSync, spawn} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import {createRequire} from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import net from 'node:net';

const require = createRequire(import.meta.url);
const sourceNamespace = process.env.JUDEX_ROUTING_SOURCE_NAMESPACE ?? 'judex';
const sourceDeployment = process.env.JUDEX_ROUTING_SOURCE_DEPLOYMENT ?? 'judex';
const clusterContext = process.env.JUDEX_ROUTING_CONTEXT ?? 'docker-desktop';
const runId = 'judex-ui-' + randomUUID().slice(0, 8), namespace = runId, release = 'routing';
const artifact = path.resolve('.cache/k8s', runId);
fs.mkdirSync(artifact, {recursive: true});
const run = (command, args, input) => execFileSync(command, args, {encoding: 'utf8', input, windowsHide: true, timeout: 240000, maxBuffer: 24 * 1024 * 1024});
const kube = (...args) => run('kubectl', ['--context', clusterContext, ...args]);
const results = {schemaVersion: 1, runId, namespace, context: clusterContext, sourceNamespace, sourceDeployment, artifact, checks: []};
let uid, owned = false, forward, exitCode = 0;
const ownership = () => {
  const current = JSON.parse(kube('get', 'namespace', namespace, '-o', 'json'));
  if (current.metadata.uid !== uid || current.metadata.labels?.['judex.dev/test-run'] !== runId) throw new Error('Namespace ownership changed');
};
const waitReady = async base => {
  const deadline = Date.now() + 60000;
  while (Date.now() < deadline) {
    try { if ((await fetch(base + '/readyz')).ok) return; } catch {}
    if (forward.exitCode !== null) throw new Error('Owned port-forward exited');
    await new Promise(resolve => setTimeout(resolve, 250));
  }
  throw new Error('Owned API readiness timeout');
};
const child = (command, args, env) => new Promise((resolve, reject) => {
  const process = spawn(command, args, {windowsHide: true, stdio: 'inherit', env});
  process.once('error', reject); process.once('exit', resolve);
});
const db = sql => JSON.parse(kube('-n', namespace, 'exec', 'statefulset/' + release + '-postgresql', '--',
  'psql', '-U', 'judex', '-d', 'judex', '-At', '-c', sql).trim());
const productionSnapshot = () => {
  const deployment = JSON.parse(kube('-n', sourceNamespace, 'get', 'deployment', sourceDeployment, '-o', 'json'));
  const tableCounts = JSON.parse(kube('-n', sourceNamespace, 'exec', 'statefulset/' + sourceDeployment + '-postgresql', '--', 'psql', '-U', 'judex', '-d', 'judex', '-At', '-c',
    "SELECT jsonb_build_object('users',(SELECT count(*) FROM users),'projects',(SELECT count(*) FROM projects),'positions',(SELECT count(*) FROM position_templates),'bindings',(SELECT count(*) FROM position_node_bindings),'workflows',(SELECT count(*) FROM workflow_definitions),'tasks',(SELECT count(*) FROM tasks),'messages',(SELECT count(*) FROM messages))").trim());
  return {deploymentUID: deployment.metadata.uid, generation: deployment.metadata.generation,
    image: deployment.spec.template.spec.containers.find(container => container.name === 'server').image, tableCounts};
};
try {
  if (run('kubectl', ['config', 'current-context']).trim() !== clusterContext) throw new Error('Image importer requires the selected current context');
  kube('get', '--raw=/readyz', '--request-timeout=10s');
  results.productionBefore = productionSnapshot();
  const source = JSON.parse(kube('-n', sourceNamespace, 'get', 'deployment', sourceDeployment, '-o', 'json')).spec.template.spec.containers.find(container => container.name === 'server');
  if (!source) throw new Error('Source server container unavailable');
  const resolveEnv = name => {
    const entry = source.env?.find(entry => entry.name === name);
    if (entry?.value !== undefined) return entry.value;
    if (entry?.valueFrom?.secretKeyRef) {
      const ref = entry.valueFrom.secretKeyRef;
      return Buffer.from(JSON.parse(kube('-n', sourceNamespace, 'get', 'secret', ref.name, '-o', 'json')).data[ref.key] ?? '', 'base64').toString();
    }
    for (const reference of source.envFrom ?? []) {
      if (!reference.secretRef) continue;
      const data = JSON.parse(kube('-n', sourceNamespace, 'get', 'secret', reference.secretRef.name, '-o', 'json')).data;
      if (data[name]) return Buffer.from(data[name], 'base64').toString();
    }
    throw new Error('Missing model setting ' + name);
  };
  const model = Object.fromEntries(['JUDEX_MODEL_PROTOCOL', 'JUDEX_MODEL_BASE_URL', 'JUDEX_MODEL_NAME', 'JUDEX_MODEL_API_KEY'].map(name => [name, resolveEnv(name)]));
  if (!model.JUDEX_MODEL_API_KEY) throw new Error('Source model credential unavailable');
  results.model = {protocol: model.JUDEX_MODEL_PROTOCOL, name: model.JUDEX_MODEL_NAME};
  const testedImage = process.env.JUDEX_ROUTING_TEST_IMAGE ?? source.image;
  const inspected = JSON.parse(run('docker', ['image', 'inspect', testedImage]))[0];
  results.sourceImage = {reference: testedImage, id: inspected.Id, deployedReference: source.image,
    scope: testedImage === source.image ? 'deployed-release' : 'explicit-workspace-build'};
  const image = 'judex/server:collaboration-routing-' + runId.slice('judex-ui-'.length);
  run('docker', ['image', 'tag', inspected.Id, image]); results.testImage = image;
  const reservation = net.createServer(); await new Promise(resolve => reservation.listen(0, '127.0.0.1', resolve));
  const port = reservation.address().port; await new Promise(resolve => reservation.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  uid = JSON.parse(run('kubectl', ['--context', clusterContext, 'create', '-f', '-', '-o', 'json'], JSON.stringify({apiVersion: 'v1', kind: 'Namespace',
    metadata: {name: namespace, labels: {'judex.dev/test-run': runId}}}))).metadata.uid;
  owned = true;
  results.namespaceUID = uid;
  run(process.execPath, ['tests/k8s/load-images.mjs', namespace, runId, uid, image]);
  ownership();
  run('kubectl', ['--context', clusterContext, 'apply', '-f', '-'], JSON.stringify({apiVersion: 'v1', kind: 'Secret',
    metadata: {name: 'routing-real-model', namespace, labels: {'judex.dev/test-run': runId}}, stringData: {key: model.JUDEX_MODEL_API_KEY}}));
  const at = image.lastIndexOf(':');
  const values = {environment: 'development', image: {repository: image.slice(0, at), tag: image.slice(at + 1), pullPolicy: 'Never'}, sandbox: {enabled: false},
    server: {mode: 'all', allowedOrigins: [base], previewOrigin: `http://localhost:${port}`, materialProjection: {enabled: false}, modelEnvSecret: 'routing-real-model',
      modelGateway: {protocol: model.JUDEX_MODEL_PROTOCOL, baseUrl: model.JUDEX_MODEL_BASE_URL, model: model.JUDEX_MODEL_NAME, apiKeyKey: 'key'}},
    postgresql: {embedded: {storage: '1Gi'}}, objectStorage: {embedded: {storage: '1Gi', maxVolumes: 2, growthCount: 1, volumeSizeLimitMB: 256}}};
  const valuesPath = path.join(artifact, 'values.json'); fs.writeFileSync(valuesPath, JSON.stringify(values, null, 2));
  console.log('Installing real-model experiment in ' + namespace);
  run('helm', ['upgrade', '--install', release, 'deploy/helm/judex', '--kube-context', clusterContext, '-n', namespace, '-f', valuesPath, '--wait', '--wait-for-jobs', '--timeout', '240s']);
  kube('-n', namespace, 'set', 'env', 'deployment/' + release, 'JUDEX_REGISTER_PER_IP=1000');
  kube('-n', namespace, 'rollout', 'status', 'deployment/' + release, '--timeout=120s');
  forward = spawn('kubectl', ['--context', clusterContext, '-n', namespace, 'port-forward', 'service/' + release, String(port) + ':8080'], {windowsHide: true, stdio: ['ignore', 'pipe', 'pipe']});
  forward.stdout.on('data', () => {}); forward.stderr.on('data', () => {}); await waitReady(base);
  results.system = (await (await fetch(base + '/api/v1/system')).json()).data;
  const cli = path.join(artifact, process.platform === 'win32' ? 'judex.exe' : 'judex'); run('go', ['build', '-o', cli, './cmd/judex']);
  results.platformExitCode = await child(process.execPath, [require.resolve('@playwright/test/cli'), 'test', '--config', 'tests/e2e/business.config.ts', '--grep', 'WR01'],
    {...process.env, JUDEX_ROUTING_LIVE: '1', JUDEX_E2E_BASE_URL: base, JUDEX_E2E_CLI: cli, JUDEX_E2E_ARTIFACT: artifact});
  if (fs.existsSync('tests/results/business')) fs.cpSync('tests/results/business', path.join(artifact, 'browser'), {recursive: true});
  if (results.platformExitCode !== 0) exitCode = 1;
  const fixturePath = path.join(artifact, 'routing-fixture.json');
  if (process.env.JUDEX_ROUTING_SKIP_DISCRIMINATION === '1') {
    results.discriminationSkipped = 'Explicit platform-only comparison run';
  } else if (fs.existsSync(fixturePath)) {
    console.log('Testing the real model with full candidate context; expectations are withheld from requests');
    results.discriminationExitCode = await child('go', ['test', './tests/agent/live', '-run', '^TestLiveWorkflowRouting$', '-count=1', '-v', '-timeout', '20m'],
      {...process.env, JUDEX_ROUTING_FIXTURE: fixturePath, JUDEX_LIVE_PROTOCOL: model.JUDEX_MODEL_PROTOCOL, JUDEX_LIVE_BASE_URL: model.JUDEX_MODEL_BASE_URL,
        JUDEX_LIVE_MODEL: model.JUDEX_MODEL_NAME, JUDEX_LIVE_API_KEY: model.JUDEX_MODEL_API_KEY});
    if (results.discriminationExitCode !== 0) exitCode = 1;
  } else { results.discriminationSkipped = 'Platform fixture setup did not complete'; exitCode = 1; }
} catch (error) {
  exitCode = 1; results.error = error.message; console.error(error.message);
} finally {
  if (owned) {
    try {
      ownership();
      const baselinePath = path.join(artifact, 'platform-baseline.json');
      if (fs.existsSync(baselinePath)) {
        const baseline = JSON.parse(fs.readFileSync(baselinePath, 'utf8'));
        if (/^[0-9a-f-]{36}$/.test(baseline.projectId)) {
          const id = baseline.projectId;
          const proof = db(`SELECT jsonb_build_object(
            'bindings',(SELECT COALESCE(jsonb_agg(jsonb_build_object('positionId',template_id,'workflowId',workflow_id,'nodeId',node_id)),'[]') FROM position_node_bindings WHERE project_id='${id}'),
            'analyses',(SELECT COALESCE(jsonb_agg(to_jsonb(a)),'[]') FROM task_analyses a WHERE a.project_id='${id}'),
            'batches',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',b.id,'taskId',b.task_id,'sourceSubmissionId',b.source_submission_id,'sourceReportId',b.source_report_id,'state',b.state)),'[]') FROM discussion_batches b WHERE b.project_id='${id}'),
            'runs',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',r.id,'batchId',r.batch_id,'identityId',r.identity_id,'parentRunId',r.parent_run_id,'state',r.state,'manifest',r.manifest)),'[]') FROM agent_runs r WHERE r.project_id='${id}'),
            'modelCalls',(SELECT COALESCE(jsonb_agg(to_jsonb(c)),'[]') FROM model_calls c WHERE c.project_id='${id}'),
            'tools',(SELECT COALESCE(jsonb_agg(jsonb_build_object('runId',c.run_id,'name',c.name,'state',c.state,'result',c.result_json)),'[]') FROM tool_calls c WHERE c.project_id='${id}'),
            'tasks',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',t.id,'title',t.title,'workflowId',t.workflow_id,'nodeId',t.node_id,'status',t.status)),'[]') FROM tasks t WHERE t.project_id='${id}'),
            'unlinkedSubmissions',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',s.id,'taskId',s.task_id,'topicId',s.topic_id,'status',s.status)),'[]') FROM submissions s WHERE s.project_id='${id}' AND s.task_id IS NULL))`);
          fs.writeFileSync(path.join(artifact, 'platform-proof.json'), JSON.stringify(proof, null, 2));
          results.platformProof = {bindings: proof.bindings.length, analyses: proof.analyses.length, modelCalls: proof.modelCalls.length,
            succeededModelCalls: proof.modelCalls.filter(call => call.state === 'succeeded').length, roleRuns: proof.runs.filter(run => run.parentRunId && run.state === 'succeeded').length};
        }
      }
      fs.writeFileSync(path.join(artifact, 'pods.json'), kube('-n', namespace, 'get', 'pods', '-o', 'json'));
      fs.writeFileSync(path.join(artifact, 'events.txt'), kube('-n', namespace, 'get', 'events', '--sort-by=.lastTimestamp'));
    } catch (error) { results.proofError = error.message; exitCode = 1; }
    if (forward?.exitCode === null) forward.kill();
    try { ownership(); console.log('Removing owned namespace ' + namespace); kube('delete', 'namespace', namespace, '--wait=true', '--timeout=120s'); results.cleanedUp = true; }
    catch (error) { results.cleanupError = error.message; exitCode = 1; }
  }
  try {
    results.productionAfter = productionSnapshot();
    results.productionUnchanged = JSON.stringify(results.productionBefore) === JSON.stringify(results.productionAfter);
  } catch (error) { results.productionSnapshotError = error.message; }
  const discrimination = path.join(artifact, 'model-routing-results.json');
  if (fs.existsSync(discrimination)) { const report = JSON.parse(fs.readFileSync(discrimination, 'utf8')); results.discrimination = {passed: report.passed, failed: report.failed, trials: report.trials.length}; }
  results.exitCode = exitCode;
  fs.writeFileSync(path.join(artifact, 'manifest.json'), JSON.stringify(results, null, 2));
  console.log('Workflow routing evidence: ' + artifact); process.exitCode = exitCode;
}
