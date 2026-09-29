// Verify packaged checksums, native CLI metadata and installer ownership.
import fs from 'node:fs';
import path from 'node:path';
import {createHash, randomUUID} from 'node:crypto';
import {execFileSync} from 'node:child_process';
const directory = path.resolve(process.argv[2] ?? '');
const manifest = JSON.parse(fs.readFileSync(path.join(directory, 'manifest.json'), 'utf8'));
let checked = 0;
for (const line of fs.readFileSync(path.join(directory, 'checksums.txt'), 'utf8').trim().split('\n')) {
  const [expected, relative] = line.trim().split('  ');
  const file = path.resolve(directory, relative);
  if (!file.startsWith(directory + path.sep)) throw new Error('Checksum path escapes release');
  const actual = createHash('sha256').update(fs.readFileSync(file)).digest('hex');
  if (actual !== expected) throw new Error('Checksum mismatch: ' + relative);
  checked++;
}
const root = path.resolve('.cache/release-tests', randomUUID());
fs.mkdirSync(root, {recursive: true});
const platform = {win32: 'windows', linux: 'linux', darwin: 'darwin'}[process.platform];
const arch = {x64: 'amd64', arm64: 'arm64'}[process.arch];
if (!platform || !arch) throw new Error('Unsupported native verification host');
const archive = path.join(directory, `judex-${platform}-${arch}-${manifest.version}.zip`);
execFileSync(process.env.JUDEX_PYTHON ?? 'python', ['-c', 'import sys,zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])', archive, root], {windowsHide: true});
const cli = path.join(root, platform === 'windows' ? 'judex.exe' : 'judex');
if (platform !== 'windows') fs.chmodSync(cli, 0o755);
const invoke = (...args) => JSON.parse(execFileSync(cli, ['--json', ...args], {cwd: root, encoding: 'utf8', windowsHide: true}));
const version = invoke('version');
if (version.data?.cli !== manifest.version || version.ok !== true) throw new Error('CLI version mismatch');
const installed = path.join(root, 'installed/judex');
invoke('skill', 'install', '--target', 'path', '--path', installed);
if (JSON.parse(fs.readFileSync(path.join(installed, 'manifest.json'))).version !== manifest.version) throw new Error('Skill version mismatch');
fs.writeFileSync(path.join(installed, 'personal-note.txt'), 'preserve');
invoke('skill', 'uninstall', '--target', 'path', '--path', installed);
if (fs.existsSync(path.join(installed, 'SKILL.md')) || fs.readFileSync(path.join(installed, 'personal-note.txt'), 'utf8') !== 'preserve') throw new Error('Installer ownership failed');
const result = {version: manifest.version, sourceTreeHash: manifest.sourceTreeHash, checksumsVerified: checked, nativePlatform: `${platform}-${arch}`, installedPackagedSkillVersionMatches: true, uninstallPreservesUserFiles: true};
fs.writeFileSync(path.join(root, 'manifest.json'), JSON.stringify(result, null, 2) + '\n');
console.log(JSON.stringify(result, null, 2));
console.log('Evidence:', root);
