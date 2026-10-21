// Build/push only the bytes from a verified, committed release candidate.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync, spawnSync} from 'node:child_process';
import {capture, imageDefinitions, sha256, validateVersion, verifyRelease, verifyReleaseSource} from './artifacts.mjs';

const [version, mode, ...extra] = process.argv.slice(2);
validateVersion(version ?? '');
if (!['--load', '--push'].includes(mode) || extra.length) throw new Error('Usage: node scripts/release/images.mjs VERSION --load|--push');
const directory = path.resolve('dist/release', version);
const manifest = verifyRelease(directory);
if (manifest.version !== version) throw new Error('Release directory version mismatch');
verifyReleaseSource(manifest);
const images = imageDefinitions(version);
if (JSON.stringify(manifest.images) !== JSON.stringify(images)) throw new Error('Release image references do not match');
const context = path.resolve('.cache/release-images', version, manifest.sourceTreeHash);
const output = path.resolve('dist/publish', version);
fs.mkdirSync(output, {recursive: true});
const statePath = path.join(output, 'images.json');
const state = fs.existsSync(statePath) ? JSON.parse(fs.readFileSync(statePath, 'utf8')) : {
  version, sourceCommit: manifest.sourceCommit, sourceTreeHash: manifest.sourceTreeHash, images: [],
};
if (state.sourceCommit !== manifest.sourceCommit || state.sourceTreeHash !== manifest.sourceTreeHash) throw new Error('Existing image publication belongs to another source');

// Copy explicit inputs only: local config, env files and credentials never enter contexts.
const copy = (source, target) => { fs.mkdirSync(path.dirname(target), {recursive: true}); fs.copyFileSync(source, target); };
const server = path.join(context, 'server');
for (const name of ['LICENSE', 'NOTICE']) copy(path.join(directory, name), path.join(server, name));
for (const arch of ['amd64', 'arm64']) copy(path.join(directory, `judex-server-linux-${arch}`), path.join(server, 'candidate', `judex-server-linux-${arch}`));
copy('deploy/docker/server.release.Dockerfile', path.join(server, 'Dockerfile'));
execFileSync(process.env.JUDEX_PYTHON ?? 'python3', ['-c', 'import sys,zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])', path.join(directory, 'web.zip'), path.join(server, 'candidate/web')]);
for (const name of ['Dockerfile', 'server.py']) copy('deploy/material-converter/' + name, path.join(context, 'material-converter', name));
copy('deploy/analysis-sandbox/Dockerfile', path.join(context, 'analysis-sandbox/Dockerfile'));
for (const name of ['requirements.txt', 'analyze_material.py']) copy('deploy/analysis-sandbox/' + name, path.join(context, 'analysis-sandbox/deploy/analysis-sandbox', name));
const architecture = {aarch64: 'arm64', arm64: 'arm64', x86_64: 'amd64', amd64: 'amd64'}[capture('docker', ['info', '--format', '{{.Architecture}}'])];
if (!architecture) throw new Error('Unsupported Docker daemon architecture');

for (const image of images) {
  verifyReleaseSource(manifest);
  if (mode === '--push') {
    const existing = spawnSync('docker', ['buildx', 'imagetools', 'inspect', image.reference, '--format', '{{json .Manifest}}'], {encoding: 'utf8', timeout: 60000});
    if (existing.status === 0) {
      const remote = JSON.parse(existing.stdout);
      const previous = state.images.find(item => item.reference === image.reference);
      if (!previous || previous.digest !== remote.digest) throw new Error('Refusing to replace an existing release tag: ' + image.reference);
      console.log('Verified previously published image:', image.reference, previous.digest);
      continue;
    }
    if (!/not found|manifest unknown|does not exist|insufficient_scope/i.test(existing.stderr ?? '')) throw new Error('Cannot verify registry tag: ' + existing.stderr);
  }
  const metadata = path.join(output, image.name + (mode === '--load' ? '-local' : '') + '.metadata.json');
  const platforms = mode === '--load' ? ['linux/' + architecture] : image.platforms;
  console.log('Building', image.reference, platforms.join(','));
  execFileSync('docker', [
    'buildx', 'build', '--platform', platforms.join(','), '--tag', image.reference,
    '--label', 'org.opencontainers.image.version=' + version,
    '--label', 'org.opencontainers.image.revision=' + manifest.sourceCommit,
    '--label', 'org.opencontainers.image.source=https://github.com/Minecodex/Judex',
    '--label', 'org.opencontainers.image.licenses=Apache-2.0',
    '--metadata-file', metadata, mode, path.join(context, image.name),
  ], {stdio: 'inherit', windowsHide: true});
  if (mode === '--push') {
    const digest = JSON.parse(fs.readFileSync(metadata, 'utf8'))['containerimage.digest'];
    if (!/^sha256:[a-f0-9]{64}$/.test(digest ?? '')) throw new Error('Missing published image digest');
    state.images.push({...image, digest});
    fs.writeFileSync(statePath, JSON.stringify(state, null, 2) + '\n');
  }
}
verifyReleaseSource(manifest);
if (mode === '--push') {
  fs.writeFileSync(statePath + '.sha256', sha256(fs.readFileSync(statePath)) + '  images.json\n');
  console.log('Published image indexes:', statePath);
} else console.log('Native candidate images loaded');
