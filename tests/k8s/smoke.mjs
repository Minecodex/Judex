// SPDX-License-Identifier: Apache-2.0
// Requires an existing compatible OpenSandbox operator and a pullable image.
import {execFileSync} from 'node:child_process';
import {randomUUID} from 'node:crypto';
const runID = randomUUID();
const namespace = 'judex-e2e-' + runID.slice(0, 8);
const image = process.env.JUDEX_TEST_IMAGE;
if (!image || !/^[\w./:-]+:[\w.-]+$/.test(image)) {
  throw new Error('Set JUDEX_TEST_IMAGE to a cluster-pullable repository:tag first');
}
const at = image.lastIndexOf(':');
const run = (command, args, input) => execFileSync(command, args, {
  encoding: 'utf8', input, stdio: ['pipe', 'pipe', 'pipe'], timeout: 900_000,
});
const kubectl = (...args) => run('kubectl', args);
// Read-only preflight before creating anything. Never create a second operator.
kubectl('get', '--raw=/readyz', '--request-timeout=10s');
kubectl('get', 'crd', 'batchsandboxes.sandbox.opensandbox.io');
const owned = [];
let installAttempted = false;
try {
  for (const name of [namespace, namespace + '-sandboxes']) {
    run('kubectl', ['create', '-f', '-'], JSON.stringify({
      apiVersion: 'v1', kind: 'Namespace', metadata: {name,
        labels: {'judex.dev/test-run': runID, 'app.kubernetes.io/managed-by': 'Helm'},
        annotations: {'meta.helm.sh/release-name': 'judex', 'meta.helm.sh/release-namespace': namespace},
      },
    }));
    owned.push(name);
  }
  installAttempted = true;
  run('helm', ['upgrade', '--install', 'judex', 'deploy/helm/judex', '-n', namespace,
    '-f', 'deploy/helm/judex/values-dev.yaml',
    '--set', 'opensandbox.opensandbox-controller.enabled=false',
    '--set-string', `image.repository=${image.slice(0, at)}`,
    '--set-string', `image.tag=${image.slice(at + 1)}`, '--wait', '--timeout', '10m']);
  const pod = JSON.parse(kubectl('-n', namespace, 'get', 'pods', '-l',
    'app.kubernetes.io/name=judex,app.kubernetes.io/instance=judex', '-o', 'json')).items[0];
  for (const path of ['/healthz', '/readyz', '/api/v1/system']) {
    const result = JSON.parse(kubectl('-n', namespace, 'exec', pod.metadata.name,
      '--', 'wget', '-qO-', 'http://127.0.0.1:8080' + path));
    const healthy = path === '/api/v1/system'
      ? result.data?.protocolVersion === '1' && ['identity', 'projects', 'persistence'].every(key => result.data.capabilities?.[key] === true)
      : ['ok', 'ready'].includes(result.status);
    if (!healthy) {
      throw new Error('Unexpected HTTP result: ' + path);
    }
  }
  console.log('Kubernetes production capability smoke passed:', namespace);
} finally {
  const cleanupErrors = [];
  if (installAttempted) {
    try { run('helm', ['uninstall', 'judex', '-n', namespace, '--wait', '--timeout', '3m']); }
    catch (error) { cleanupErrors.push(error); }
  }
  for (const name of owned.reverse()) {
    try {
      const current = JSON.parse(kubectl('get', 'namespace', name, '-o', 'json'));
      if (!name.startsWith('judex-e2e-') || current.metadata.labels?.['judex.dev/test-run'] !== runID) {
        throw new Error('Ownership changed; refusing namespace cleanup: ' + name);
      }
      kubectl('delete', 'namespace', name, '--wait=true', '--timeout=3m');
    } catch (error) { cleanupErrors.push(error); }
  }
  if (cleanupErrors.length) throw new AggregateError(cleanupErrors, 'Check cleanup for ' + namespace);
}
