// Real storage combinations and bounded faults in ownership-labelled namespaces.
import {execFileSync, spawn} from 'node:child_process';
import {randomUUID, randomBytes, createHash} from 'node:crypto';
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';

const image = process.env.JUDEX_TEST_IMAGE;
if (!image || !/^[\w./:-]+:[\w.-]+$/.test(image)) throw new Error('Set JUDEX_TEST_IMAGE to the acceptance image containing judex-server and mock-gateway');
const runId = 'judex-close-' + randomUUID().slice(0, 8), release = 'matrix';
const artifact = path.resolve('.cache/k8s', runId); fs.mkdirSync(artifact, {recursive: true});
const previewReservation=net.createServer();await new Promise(resolve=>previewReservation.listen(0,'127.0.0.1',resolve));const previewPort=previewReservation.address().port;
const owned = [], forwards = new Set(), results = [], failures = [];
const run = (command, args, input) => String(execFileSync(command, args, {input, encoding: 'utf8', windowsHide: true, timeout: 600000, maxBuffer: 16 << 20})).trim();
const kube = (...args) => run('kubectl', args);
const apply = (object) => run('kubectl', ['apply', '-f', '-'], JSON.stringify(object));
const pause = (ms) => new Promise(resolve => setTimeout(resolve, ms));
async function until(check, label, timeout = 180000) { const end = Date.now() + timeout; for (;;) { try { if (await check()) return; } catch {} if (Date.now() >= end) throw new Error(label + ' timed out'); await pause(500); } }
function ownership(ns) { const data = JSON.parse(kube('get', 'ns', ns, '-o', 'json')); if (data.metadata.labels?.['judex.test-run'] !== runId) throw new Error('Ownership changed: ' + ns); }
function createNamespace(ns, parent = ns) { apply({apiVersion: 'v1', kind: 'Namespace', metadata: {name: ns, labels: {'judex.test-run': runId, 'app.kubernetes.io/managed-by': 'Helm'}, annotations: {'meta.helm.sh/release-name': release, 'meta.helm.sh/release-namespace': parent}}}); owned.push(ns); }
function copySecret(source, destination, name, target) { const data = JSON.parse(kube('-n', source, 'get', 'secret', name, '-o', 'json')); apply({apiVersion: 'v1', kind: 'Secret', metadata: {name: target, namespace: destination}, type: 'Opaque', data: data.data}); }
async function forward(ns, service, preferredPort=0) {
  const listener = net.createServer(); await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve)); const port = preferredPort || listener.address().port; await new Promise(resolve => listener.close(resolve));
  const child = spawn('kubectl', ['-n', ns, 'port-forward', 'service/' + service, `${port}:${service === 'controlled-model' ? 8081 : 8080}`], {windowsHide: true, stdio: ['ignore', 'pipe', 'pipe']}); forwards.add(child);
  let ready = false; child.stdout.on('data', chunk => { if (String(chunk).includes('Forwarding from')) ready = true; }); child.stderr.on('data', () => {});
  await until(() => ready && child.exitCode == null, 'port forward', 20000);
  return {base: `http://127.0.0.1:${port}`, close: () => { child.kill(); forwards.delete(child); }};
}
function client(base) {
  let cookies = '', csrf = '';
  const request = async (route, body, method = body === undefined ? 'GET' : 'POST', extra = {}) => {
    const res = await fetch(base.value + '/api/v1' + route, {method, signal: AbortSignal.timeout(30000), headers: {'Content-Type': 'application/json', Cookie: cookies, 'X-CSRF-Token': csrf, 'Idempotency-Key': randomUUID(), ...extra}, body: body === undefined ? undefined : Buffer.isBuffer(body) ? body : JSON.stringify(body)});
    const set = res.headers.getSetCookie(); if (set.length) cookies = set.map(c => c.split(';')[0]).join('; ');
    return res;
  };
  const json = async (...args) => { const res = await request(...args); const data = await res.json(); if (!res.ok) throw new Error(`${args[0]}: ${res.status} ${JSON.stringify(data)}`); return data.data; };
  return {json, request, authenticate: async () => { await json('/auth/register', {displayName: 'Storage acceptance', email: randomUUID() + '@judex.test', password: 'acceptance-password-123'}); const session = await json('/auth/session'); csrf = session.csrfToken; return session.user; }};
}
async function seed(ns) {
  let pf = await forward(ns, release); const base = {value: pf.base}; await until(async () => (await fetch(base.value + '/readyz')).ok, 'API readiness');
  const api = client(base), user = await api.authenticate(); const project = await api.json('/projects', {title: 'Storage and recovery acceptance'}); const prefix = `/projects/${project.id}`;
  const position = await api.json(prefix + '/positions', {name: 'Responsible', prompt: 'Review evidence'}); const identity = await api.json(prefix + '/identities', {positionId: position.id, userId: user.id});
  const proposal = await api.json(prefix + '/proposals', {kind: 'work_arrangement', changes: [{operation: 'create_task', targetType: 'task', clientRef: 'work', fields: {title: 'Preserved accepted task', expectedOutput: 'Evidence', acceptanceCriteria: 'Exact recovery', reviewerIdentityId: identity.id, participantIdentityIds: [identity.id]}}]});
  const draft = await api.json(`${prefix}/proposals/${proposal.id}/review`); const review = await api.json(`${prefix}/proposals/${proposal.id}/submit`, {expectedVersion: 1, draftHash: draft.reviewHash});
  const result = await api.json(`${prefix}/proposals/${proposal.id}/decisions`, {reviewId: review.reviewId, reviewHash: review.reviewHash, expectedVersion: 2, decision: 'approve', slotIds: review.slots.map(s => s.id), actingBindingVersions: review.slots.filter(s => s.authorityType === 'identity').map(s => ({identityId: s.authorityId, bindingVersion: s.bindingVersion}))});
  const taskId = result.createdIds.work; const taskPath = `${prefix}/tasks/${taskId}`;
  await api.json(taskPath + '/reports', {kind: 'delivery', text: 'Immutable acceptance evidence', identityId: identity.id, expectedTaskVersion: 1});
  const evidence = await api.json(taskPath + '/acceptance-review'); const accepted = await api.json(taskPath + '/acceptances', {reviewId: evidence.reviewId, reviewHash: evidence.reviewHash, expectedVersion: evidence.targetVersion, decision: 'accept'});
  const content = Buffer.from('中文材料：只记录事实，由人验收。\n'.repeat(1000)); const sha = createHash('sha256').update(content).digest('hex');
  const upload = await api.json(prefix + '/uploads', {name: 'evidence.txt', kind: 'file', size: content.length, sha256: sha, mime: 'text/plain'});
  await api.json(`${prefix}/uploads/${upload.id}/parts/1`, content, 'PUT', {'Content-Type': 'application/octet-stream', 'X-Judex-Part-SHA256': sha});
  const material = await api.json(`${prefix}/uploads/${upload.id}/complete`, {}); const download = `${prefix}/materials/${material.materialId}/versions/${material.id}/content`;
  const verify = async () => { const task = await api.json(taskPath); if (task.status !== 'accepted' || task.latestAcceptanceId !== accepted.latestAcceptanceId) throw new Error('Acceptance changed across recovery'); const res = await api.request(download); if (!res.ok || createHash('sha256').update(Buffer.from(await res.arrayBuffer())).digest('hex') !== sha) throw new Error('Restored material mismatch'); };
  await verify();
  return {ns, api, prefix, projectId: project.id, taskId, acceptanceId: accepted.latestAcceptanceId, materialVersionId: material.id, sha256: sha, download, verify, base, reconnect: async () => { pf.close(); pf = await forward(ns, release); base.value = pf.base; await until(async () => (await fetch(base.value + '/readyz')).ok, 'reconnected API'); }};
}
try {
  kube('get', '--raw=/readyz', '--request-timeout=10s');
  kube('get', 'crd', 'batchsandboxes.sandbox.opensandbox.io');
  const root = runId + '-ii'; createNamespace(root); createNamespace(root + '-sandboxes', root);
  apply({apiVersion: 'v1', kind: 'Secret', metadata: {namespace: root, name: 'controlled-key'}, stringData: {JUDEX_FAULT_KEY: randomBytes(16).toString('hex')}});
  apply({apiVersion: 'apps/v1', kind: 'Deployment', metadata: {namespace: root, name: 'controlled-model'}, spec: {replicas: 1, selector: {matchLabels: {app: 'controlled-model'}}, template: {metadata: {labels: {app: 'controlled-model'}}, spec: {automountServiceAccountToken: false, containers: [{name: 'model', image, imagePullPolicy: 'Never', command: ['/app/mock-gateway'], ports: [{containerPort: 8081}], resources: {requests: {cpu: '20m', memory: '16Mi'}, limits: {cpu: '200m', memory: '64Mi'}}}]}}}});
  apply({apiVersion: 'v1', kind: 'Service', metadata: {namespace: root, name: 'controlled-model'}, spec: {selector: {app: 'controlled-model'}, ports: [{port: 8081, targetPort: 8081}]}});
  const cases = [];
  for (const combination of ['ii', 'ie', 'ei', 'ee']) {
    const ns = runId + '-' + combination; if (ns !== root) createNamespace(ns);
    const externalPG = combination[0] === 'e', externalS3 = combination[1] === 'e';
    const values = {image: {repository: image.slice(0, image.lastIndexOf(':')), tag: image.slice(image.lastIndexOf(':') + 1), pullPolicy: 'Never'}, sandbox: {enabled: ns === root}, opensandbox: {'opensandbox-controller': {enabled: false}}, server: {mode: 'all'}, postgresql: {embedded: {enabled: !externalPG}}, objectStorage: {embedded: {enabled: !externalS3}}};
    if (ns === root) values.server = {previewOrigin:`http://localhost:${previewPort}`,mode: 'all', modelEnvSecret: 'controlled-key', modelGateway: {protocol: 'openai-compatible', baseUrl: 'http://controlled-model:8081/v1', model: 'fault-fixture', apiKeyKey: 'JUDEX_FAULT_KEY'}};
    if (externalPG) { copySecret(root, ns, release + '-postgresql', 'external-pg'); const database = 'closure_' + combination; kube('-n', root, 'exec', release + '-postgresql-0', '--', 'createdb', '-U', 'judex', database); values.postgresql = {embedded: {enabled: false}, existingSecret: 'external-pg', database, external: {host: `${release}-postgresql.${root}.svc.cluster.local`, port: 5432, sslMode: 'disable'}}; }
    if (externalS3) { copySecret(root, ns, 'judex-storage-auth', 'external-s3'); const bucket = 'closure-' + combination; kube('-n', root, 'exec', 'deployment/' + release, '--', 'env', 'JUDEX_S3_BUCKET=' + bucket, '/app/judex-server', 'objects', 'init'); values.objectStorage = {embedded: {enabled: false}, external: {endpoint: `http://${release}-seaweedfs.${root}.svc.cluster.local:8333`, region: 'us-east-1', bucket, existingSecret: 'external-s3'}}; }
    const file = path.join(artifact, combination + '.json'); fs.writeFileSync(file, JSON.stringify(values));
    console.log('Installing storage combination', combination, ns);
    run('helm', ['upgrade', '--install', release, 'deploy/helm/judex', '-n', ns, '-f', file, '--wait', '--wait-for-jobs', '--timeout', '5m']);
    if(ns===root){await new Promise(resolve=>previewReservation.close(resolve));await forward(ns,release,previewPort);}
    const state = await seed(ns); cases.push(state); results.push({combination, namespace: ns, projectId: state.projectId, taskId: state.taskId, acceptanceId: state.acceptanceId, materialVersionId: state.materialVersionId, sha256: state.sha256, initialReadVerified: true});
    console.log('Storage combination passed', combination);
  }
  const main = cases[0]; ownership(root);
  const gateway = await forward(root, 'controlled-model'); const setMode = mode => fetch(gateway.base + '/mode/' + mode, {method: 'POST'});
  const topic = (await main.api.json(main.prefix + '/topics')).items[0];
  const start = async () => { const submission = await main.api.json(main.prefix + '/submissions', {clientSubmissionId: randomUUID(), purpose: 'message', topicId: topic.id, text: 'Controlled fault acceptance. Never approve business work.',materialVersionIds:[main.materialVersionId]}); return main.api.json(`${main.prefix}/topics/${topic.id}/runs`, {sourceSubmissionId: submission.id}); };
  const stateOf = id => main.api.json(`${main.prefix}/runs/${id}`);
  for (const mode of ['normal', 'disconnect', 'quota']) { console.log('Injecting model mode', mode); await setMode(mode); const run = await start(); let state; await until(async () => { state = await stateOf(run.runId); return ['succeeded', 'failed', 'cancelled', 'waiting_human'].includes(state.state); }, 'model ' + mode); if (mode === 'normal' ? state.state !== 'succeeded' : state.state === 'succeeded') throw new Error('Model fault reported false success'); results.push({fault: 'model-' + mode, runId: run.runId, state: state.state}); }

  console.log('Checking sandbox write and artifact publication');await setMode('publish');const publication=await start();let published;
  await until(async()=>{published=await stateOf(publication.runId);return ['succeeded','failed','waiting_human'].includes(published.state);},'artifact publication');
  if(published.state!=='succeeded'||published.artifactRefs.length!==1)throw new Error('Artifact was not registered: '+JSON.stringify(published));
  const artifactRef=published.artifactRefs[0];const preview=await main.api.json(`${main.prefix}/materials/${artifactRef.materialId}/versions/${artifactRef.versionId}/preview-session`,{});
  const {chromium}=await import('@playwright/test');const browser=await chromium.launch({channel:process.env.PLAYWRIGHT_CHANNEL??'msedge',headless:true});try{const page=await browser.newPage();await page.goto(preview.previewUrl);await page.getByRole('heading',{name:'中文发布验收',exact:true}).waitFor();if(await page.locator('html').getAttribute('data-ready')!=='yes')throw new Error('Published HTML script did not run');await page.screenshot({path:path.join(artifact,'published-html.png')});}finally{await browser.close()}
  results.push({scenario:'sandbox-publish-browser',runId:publication.runId,artifact:artifactRef,browserRendered:true});await setMode('normal');
  console.log('Injecting API restart'); kube('-n', root, 'rollout', 'restart', 'deployment/' + release); kube('-n', root, 'rollout', 'status', 'deployment/' + release, '--timeout=120s'); await main.reconnect(); await main.verify(); results.push({fault: 'api-restart', result: 'accepted task and material unchanged'});
  console.log('Injecting PostgreSQL outage'); kube('-n', root, 'scale', 'statefulset/' + release + '-postgresql', '--replicas=0'); kube('-n', root, 'wait', '--for=delete', 'pod/' + release + '-postgresql-0', '--timeout=90s');
  await until(async () => !(await fetch(main.base.value + '/readyz', {signal: AbortSignal.timeout(10000)})).ok, 'PG readiness failure');
  kube('-n', root, 'scale', 'statefulset/' + release + '-postgresql', '--replicas=1'); kube('-n', root, 'rollout', 'status', 'statefulset/' + release + '-postgresql', '--timeout=120s'); await until(async () => { await main.verify(); return true; }, 'PG recovery'); results.push({fault: 'postgres-outage', result: 'readiness failed then data verified'});
  console.log('Injecting S3 outage'); kube('-n', root, 'scale', 'statefulset/' + release + '-seaweedfs', '--replicas=0'); kube('-n', root, 'wait', '--for=delete', 'pod/' + release + '-seaweedfs-0', '--timeout=90s');
  const unavailable = await main.api.request(main.download); if (unavailable.status !== 503) throw new Error('S3 outage must fail before streaming, status=' + unavailable.status);
  kube('-n', root, 'scale', 'statefulset/' + release + '-seaweedfs', '--replicas=1'); kube('-n', root, 'rollout', 'status', 'statefulset/' + release + '-seaweedfs', '--timeout=150s'); await until(async () => { await main.verify(); return true; }, 'S3 recovery'); results.push({fault: 's3-outage', result: '503 then exact material SHA256 restored'});
  console.log('Injecting worker loss with prepared sandbox command'); await setMode('toolstall'); const interrupted = await start();
  const sql = query => kube('-n', root, 'exec', release + '-postgresql-0', '--', 'psql', '-U', 'judex', '-d', 'judex', '-Atc', query);
  await until(() => Number(sql(`SELECT count(*) FROM tool_calls WHERE run_id='${interrupted.runId}' AND name='bash' AND state='prepared'`)) > 0, 'prepared tool');
  const pods = JSON.parse(kube('-n', root, 'get', 'pods', '-l', 'app.kubernetes.io/name=judex,app.kubernetes.io/instance=' + release, '-o', 'json')).items.filter(p => p.metadata.ownerReferences?.some(o => o.kind === 'ReplicaSet'));
  ownership(root); for (const pod of pods) kube('-n', root, 'delete', 'pod', pod.metadata.name, '--grace-period=0', '--force');
  kube('-n', root, 'rollout', 'status', 'deployment/' + release, '--timeout=150s'); await main.reconnect();
  await until(async () => (await stateOf(interrupted.runId)).state === 'waiting_human', 'unknown result reconciliation', 180000);
  const unknown = Number(sql(`SELECT count(*) FROM tool_calls WHERE run_id='${interrupted.runId}' AND state='unknown'`)); if (unknown !== 1) throw new Error('Unknown command was not preserved exactly once');
  results.push({fault: 'worker-sandbox-interruption', runId: interrupted.runId, unknownTools: unknown, result: 'waiting_human; no shell replay'});
  console.log('Injecting OpenSandbox service outage');const sandboxDeployment=`${root}-${release}-opensandbox-server`.slice(0,63).replace(/-$/,'');
  ownership(root);kube('-n',root,'scale','deployment/'+sandboxDeployment,'--replicas=0');
  await until(()=>JSON.parse(kube('-n',root,'get','deployment',sandboxDeployment,'-o','json')).status.availableReplicas!==1,'sandbox unavailable');
  await setMode('toolstall');const unavailableRun=await start();await until(async()=>['waiting_human','failed'].includes((await stateOf(unavailableRun.runId)).state),'sandbox outage state');
  kube('-n',root,'scale','deployment/'+sandboxDeployment,'--replicas=1');kube('-n',root,'rollout','status','deployment/'+sandboxDeployment,'--timeout=150s');
  const sandboxProof=JSON.parse(kube('-n',root,'exec','deployment/'+release,'--','/app/judex-server','verify-sandbox',main.projectId));
  results.push({fault:'sandbox-service-outage',runId:unavailableRun.runId,recovery:sandboxProof});
  for (const state of cases) await state.verify();
  if(process.env.JUDEX_LIVE_SOURCE_NS){const {liveAcceptance}=await import('./live.mjs');await liveAcceptance({sourceNamespace:process.env.JUDEX_LIVE_SOURCE_NS,kube,apply,ownership,root,release,main,previewPort,forward,forwards,artifact,results,until});}
  console.log('Storage and fault matrix passed');
} catch (error) {
  failures.push(error.message); process.exitCode = 1; console.error(error.message);
  for (const ns of owned) { try { const pods = JSON.parse(kube('-n',ns,'get','pods','-o','json')).items;
    fs.writeFileSync(path.join(artifact,ns+'-pods.json'),JSON.stringify(pods.map(p=>({name:p.metadata.name,status:p.status})),null,2));
    for (const pod of pods) { try { fs.writeFileSync(path.join(artifact,ns+'-'+pod.metadata.name+'.log'),kube('-n',ns,'logs',pod.metadata.name,'--all-containers=true','--tail=150')); } catch {} }
  } catch {} }
}
finally {
  if(previewReservation.listening)previewReservation.close();
  for (const child of forwards) child.kill();
  for (const ns of [...owned].reverse()) { try { ownership(ns); kube('delete', 'namespace', ns, '--wait=false'); } catch (error) { failures.push('Cleanup: ' + error.message); process.exitCode = 1; } }
  fs.writeFileSync(path.join(artifact, 'manifest.json'), JSON.stringify({runId, image, results, failures, cleanupRequested: owned}, null, 2));
  console.log('Evidence:', artifact);
}
