import fs from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';

export const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
export const capture = (command, args) => String(execFileSync(command, args, {encoding: 'utf8', windowsHide: true})).trim();

export function validateVersion(version) {
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$/.test(version)) {
    throw new Error('Expected a SemVer release version without a leading v or build metadata');
  }
  return version;
}

export function imageDefinitions(version) {
  validateVersion(version);
  return ['server', 'material-converter', 'analysis-sandbox'].map(name => ({
    name,
    reference: `docker.io/kakj/judex-${name}:v${version}`,
    platforms: ['linux/amd64', 'linux/arm64'],
  }));
}

export function sourceFingerprint() {
  const records = [];
  const visit = file => {
    const info = fs.lstatSync(file);
    if (info.isSymbolicLink()) throw new Error('Release input symlink: ' + file);
    if (info.isDirectory()) {
      for (const name of fs.readdirSync(file).sort()) visit(path.join(file, name));
    } else records.push(file.replaceAll('\\', '/') + '\0' + sha256(fs.readFileSync(file)));
  };
  for (const input of [
    'cmd', 'internal', 'pkg', 'web/src', 'web/index.html', 'web/package.json',
    'web/vite.config.ts', 'web/tsconfig.json', 'api', 'skills', 'deploy/helm/judex',
    'deploy/docker', 'deploy/material-converter', 'deploy/analysis-sandbox',
    'deploy/models.beta.example.yaml', 'scripts/release', 'go.mod', 'go.sum',
    'package.json', 'package-lock.json', 'LICENSE', 'NOTICE', 'README.md',
    'docs/releases', 'docs/运维/releases.md',
  ]) visit(input);
  return sha256(Buffer.from(records.sort().join('\n')));
}

export function verifyRelease(directory) {
  const manifest = JSON.parse(fs.readFileSync(path.join(directory, 'manifest.json'), 'utf8'));
  validateVersion(manifest.version);
  const expectedFiles = new Set([...manifest.artifacts, 'manifest.json']);
  const checked = new Set();
  for (const line of fs.readFileSync(path.join(directory, 'checksums.txt'), 'utf8').trim().split('\n')) {
    const match = /^([a-f0-9]{64})  ([^/\\\r\n]+)$/.exec(line);
    if (!match || !expectedFiles.has(match[2]) || checked.has(match[2])) throw new Error('Invalid release checksum entry');
    const [, expected, name] = match;
    const file = path.join(directory, name);
    if (!fs.lstatSync(file).isFile() || sha256(fs.readFileSync(file)) !== expected) throw new Error('Checksum mismatch: ' + name);
    checked.add(name);
  }
  if (checked.size !== expectedFiles.size) throw new Error('Incomplete release checksums');
  return manifest;
}

export function verifyReleaseSource(manifest) {
  if (manifest.commit.endsWith('-dirty') || capture('git', ['status', '--porcelain'])) throw new Error('Images require a clean, committed release source');
  if (capture('git', ['rev-parse', 'HEAD']) !== manifest.sourceCommit || sourceFingerprint() !== manifest.sourceTreeHash) {
    throw new Error('Source does not match the immutable release package');
  }
}
