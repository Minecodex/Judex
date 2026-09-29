// Portable release builder. No publishing or installation side effects.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
const version = process.argv[2] ?? '0.1.0-dev';
if (!/^[0-9A-Za-z][0-9A-Za-z.+-]*$/.test(version)) throw new Error('Invalid release version');
const out = path.resolve('dist/release', version);
if (fs.existsSync(out)) throw new Error('Release directory already exists; use a new version');
const run = (cmd, args, options = {}) => execFileSync(cmd, args, {windowsHide: true, stdio: 'inherit', ...options});
const capture = (cmd, args) => String(execFileSync(cmd, args, {windowsHide: true, encoding: 'utf8'})).trim();
const commit = capture('git', ['rev-parse', '--short', 'HEAD']) + (capture('git', ['status', '--porcelain']) ? '-dirty' : '');
const flags = `-s -w -X github.com/kakj-go/Judex/internal/version.Version=${version} -X github.com/kakj-go/Judex/internal/version.Commit=${commit}`;
function sourceFingerprint(){
 const records=[];const visit=(file)=>{const info=fs.lstatSync(file);if(info.isSymbolicLink())throw new Error('Release input symlink: '+file);if(info.isDirectory()){for(const name of fs.readdirSync(file).sort())visit(path.join(file,name));}else records.push(file.replaceAll('\\','/')+'\0'+createHash('sha256').update(fs.readFileSync(file)).digest('hex'));};
 for(const input of ['cmd','internal','pkg','web/src','web/index.html','web/package.json','web/vite.config.ts','web/tsconfig.json','api','skills','deploy/helm/judex','scripts/release','go.mod','go.sum','package.json','package-lock.json','LICENSE','NOTICE'])visit(input);
 return createHash('sha256').update(records.sort().join('\n')).digest('hex');
}
const sourceTreeHash=sourceFingerprint();
fs.mkdirSync(out, {recursive: true});
const npm = process.env.npm_execpath;
if (npm) run(process.execPath, [npm, 'run', 'build']);
else run(process.platform === 'win32' ? 'npm.cmd' : 'npm', ['run', 'build'], {shell: process.platform === 'win32'});
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
run('helm', ['package', 'deploy/helm/judex', '--destination', out, '--version', version, '--app-version', version]); entries.push(`judex-${version}.tgz`);
fs.copyFileSync('api/openapi.yaml', path.join(out, 'openapi.yaml')); entries.push('openapi.yaml');
fs.copyFileSync('LICENSE', path.join(out, 'LICENSE')); fs.copyFileSync('NOTICE', path.join(out, 'NOTICE'));
if(sourceFingerprint()!==sourceTreeHash)throw new Error("Release inputs changed during build; build again from a stable tree");
const manifest = {version, commit, sourceTreeHash, protocolVersion: '1', components: ['server', 'web', 'cli', 'skill', 'chart', 'openapi'], artifacts: entries};
fs.writeFileSync(path.join(out, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
const files = [];
function walk(directory) { for (const item of fs.readdirSync(directory, {withFileTypes: true})) { const target = path.join(directory, item.name); if (item.isDirectory()) walk(target); else files.push(target); } }
walk(out);
const checksums = files.sort().map(file => createHash('sha256').update(fs.readFileSync(file)).digest('hex') + '  ' + path.relative(out, file).replaceAll('\\', '/'));
fs.writeFileSync(path.join(out, 'checksums.txt'), checksums.join('\n') + '\n');
for (const line of checksums) { const [expected, relative] = line.split('  '); const actual = createHash('sha256').update(fs.readFileSync(path.join(out, relative))).digest('hex'); if (actual !== expected) throw new Error('Checksum verification failed: ' + relative); }
console.log('Release built and checksums verified:', out);
