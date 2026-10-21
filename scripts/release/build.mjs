// Portable release builder. No publishing or installation side effects.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import yaml from 'js-yaml';
import {capture, imageDefinitions, sha256, sourceFingerprint, validateVersion} from './artifacts.mjs';
const version = process.argv[2] ?? '0.0.1-beta';
validateVersion(version);
const out = path.resolve('dist/release', version);
if (fs.existsSync(out)) throw new Error('Release directory already exists; use a new version');
const run = (cmd, args, options = {}) => execFileSync(cmd, args, {windowsHide: true, stdio: 'inherit', ...options});
const sourceCommit = capture('git', ['rev-parse', 'HEAD']);
const commit = capture('git', ['rev-parse', '--short', 'HEAD']) + (capture('git', ['status', '--porcelain']) ? '-dirty' : '');
const flags = `-s -w -X github.com/kakj-go/Judex/internal/version.Version=${version} -X github.com/kakj-go/Judex/internal/version.Commit=${commit}`;
const sourceTreeHash=sourceFingerprint();
fs.mkdirSync(out, {recursive: true});
const npm = process.env.npm_execpath;
if (npm) run(process.execPath, [npm, 'run', 'build'], {env:{...process.env,JUDEX_RELEASE_VERSION:version}});
else run(process.platform === 'win32' ? 'npm.cmd' : 'npm', ['run', 'build'], {shell: process.platform === 'win32',env:{...process.env,JUDEX_RELEASE_VERSION:version}});
const entries = [];
const skillStage=path.join(out,'skill');fs.cpSync('skills/judex',skillStage,{recursive:true});
const skillManifest=JSON.parse(fs.readFileSync(path.join(skillStage,'manifest.json'),'utf8'));skillManifest.version=version;fs.writeFileSync(path.join(skillStage,'manifest.json'),JSON.stringify(skillManifest,null,2)+'\n');

function pack(source, destination) {
  run(process.env.JUDEX_PYTHON ?? 'python', ['-c', 'import sys,pathlib,zipfile; root=pathlib.Path(sys.argv[1]); z=zipfile.ZipFile(sys.argv[2],"w",zipfile.ZIP_DEFLATED); [(z.write(p,p.relative_to(root).as_posix())) for p in sorted(root.rglob("*")) if p.is_file()]; z.close()', source, destination]);
}
for (const [goos, arch] of [['windows', 'amd64'], ['windows', 'arm64'], ['linux', 'amd64'], ['linux', 'arm64'], ['darwin', 'amd64'], ['darwin', 'arm64']]) {
  const directory = path.join(out, 'cli', `${goos}-${arch}`); fs.mkdirSync(directory, {recursive: true});
  const executable = path.join(directory, goos === 'windows' ? 'judex.exe' : 'judex');
  run('go', ['build', '-trimpath', '-ldflags', flags, '-o', executable, './cmd/judex'], {env: {...process.env, GOOS: goos, GOARCH: arch, CGO_ENABLED: '0'}});
  fs.cpSync(skillStage, path.join(directory, 'skills/judex'), {recursive: true});
  const archive = path.join(out, `judex-${goos}-${arch}-${version}.zip`); pack(directory, archive); entries.push(path.basename(archive));
  console.log('Built CLI', goos, arch);
}
for (const arch of ['amd64', 'arm64']) {
  const target = `judex-server-linux-${arch}`;
  run('go', ['build', '-trimpath', '-ldflags', flags, '-o', path.join(out, target), './cmd/judex-server'], {env: {...process.env, GOOS: 'linux', GOARCH: arch, CGO_ENABLED: '0'}});
  entries.push(target);
}
pack('web/dist', path.join(out, 'web.zip')); entries.push('web.zip');
pack(skillStage, path.join(out, 'judex-skill.zip')); entries.push('judex-skill.zip');
fs.copyFileSync('api/openapi.yaml', path.join(out, 'openapi.yaml')); entries.push('openapi.yaml');
fs.copyFileSync('LICENSE', path.join(out, 'LICENSE')); fs.copyFileSync('NOTICE', path.join(out, 'NOTICE'));
entries.push('LICENSE', 'NOTICE');
for (const [source, name] of [['README.md', 'README.md'], ['deploy/models.beta.example.yaml', 'models.example.yaml']]) {
  fs.copyFileSync(source, path.join(out, name)); entries.push(name);
}
const images = imageDefinitions(version);
const values = yaml.load(fs.readFileSync('deploy/helm/judex/values-beta.yaml', 'utf8'));
values.image.repository = images[0].reference.slice(0, images[0].reference.lastIndexOf(':'));
values.image.tag = 'v' + version;
values.server.materialPreview.image = images[1].reference;
values.server.sandboxImage = images[2].reference;
fs.writeFileSync(path.join(out, 'values-beta.yaml'), yaml.dump(values)); entries.push('values-beta.yaml');
const chartStage = path.join(out, 'chart'); fs.cpSync('deploy/helm/judex', chartStage, {recursive: true});
const defaultsPath = path.join(chartStage, 'values.yaml');
const defaults = yaml.load(fs.readFileSync(defaultsPath, 'utf8'));
defaults.image.repository = values.image.repository;
defaults.image.tag = values.image.tag;
defaults.server.materialPreview.image = values.server.materialPreview.image;
defaults.server.sandboxImage = values.server.sandboxImage;
fs.writeFileSync(defaultsPath, yaml.dump(defaults));
fs.writeFileSync(path.join(chartStage, 'values-beta.yaml'), yaml.dump(values));
run('helm', ['package', chartStage, '--destination', out, '--version', version, '--app-version', version]); entries.push(`judex-${version}.tgz`);
const notes = `docs/releases/v${version}.md`;
if (fs.existsSync(notes)) { fs.copyFileSync(notes, path.join(out, 'release-notes.md')); entries.push('release-notes.md'); }
if(sourceFingerprint()!==sourceTreeHash)throw new Error("Release inputs changed during build; build again from a stable tree");
const manifest = {version, commit, sourceCommit, sourceTreeHash, protocolVersion: '2', components: ['server', 'web', 'cli', 'skill', 'chart', 'openapi'], images, artifacts: entries};
fs.writeFileSync(path.join(out, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
const checksums = [...entries, 'manifest.json'].sort().map(name => sha256(fs.readFileSync(path.join(out, name))) + '  ' + name);
fs.writeFileSync(path.join(out, 'checksums.txt'), checksums.join('\n') + '\n');
for (const line of checksums) { const [expected, relative] = line.split('  '); if (sha256(fs.readFileSync(path.join(out, relative))) !== expected) throw new Error('Checksum verification failed: ' + relative); }
console.log('Release built and checksums verified:', out);
