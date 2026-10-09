// Read-only host discovery plus real installer lifecycle. No model turns.
import {execFileSync, spawn} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
const root = path.resolve('.cache/skill-hosts', randomUUID()); fs.mkdirSync(root, {recursive: true});
const cli = path.join(root, process.platform === 'win32' ? 'judex.exe' : 'judex');
execFileSync('go', ['build', '-o', cli, './cmd/judex'], {windowsHide: true, stdio: 'inherit'});
const invoke = (...args) => JSON.parse(execFileSync(cli, ['--json', ...args], {encoding: 'utf8', windowsHide: true, env: {...process.env, JUDEX_CONFIG_DIR: path.join(root, 'config')}}));
const codexSkill = path.join(root, '.agents/skills/judex');
const claudeSkill = path.join(root, '.claude/skills/judex');
invoke('skill', 'install', '--target', 'path', '--path', codexSkill);
invoke('skill', 'install', '--target', 'path', '--path', claudeSkill);
for (const directory of [codexSkill, claudeSkill]) {
  const raw = fs.readFileSync(path.join(directory, 'SKILL.md'), 'utf8').replaceAll('\r\n','\n');
  if (!raw.startsWith('---\nname: judex\n') || !raw.includes('description:')) throw new Error('Missing skill metadata');
}
const entry = process.env.JUDEX_CODEX_ENTRY;
const server = spawn(entry ? process.execPath : 'codex', [...(entry ? [entry] : []), 'app-server', '--listen', 'stdio://'], {cwd: root, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe']});
const pending = new Map(); let buffer = '', nextId = 1;
server.stderr.on('data', () => {});
server.stdout.on('data', chunk => {
  buffer += chunk; for (;;) { const end = buffer.indexOf('\n'); if (end < 0) break; const line = buffer.slice(0, end); buffer = buffer.slice(end + 1); let response; try { response = JSON.parse(line); } catch { continue; } const waiter = pending.get(response.id); if (!waiter) continue; pending.delete(response.id); clearTimeout(waiter.timer); response.error ? waiter.reject(new Error(response.error.message)) : waiter.resolve(response.result); }
});
function rpc(method, params) { return new Promise((resolve, reject) => { const id = nextId++; const timer = setTimeout(() => { pending.delete(id); reject(new Error(method + ' timed out')); }, 30000); pending.set(id, {resolve, reject, timer}); server.stdin.write(JSON.stringify({id, method, params}) + '\n'); }); }
const evidence = {root, codexDiscovery: false, claudeLayoutValidated: true, installerPreservesUserFiles: false};
try {
  await rpc('initialize', {clientInfo: {name: 'judex-skill-acceptance', version: '1'}});
  server.stdin.write(JSON.stringify({method: 'initialized'}) + '\n');
  const listed = await rpc('skills/list', {cwds: [root], forceReload: true});
  const found = listed.data.flatMap(entry => entry.skills ?? []).find(skill => skill.name === 'judex' && String(skill.path).replaceAll('\\', '/').includes('/.agents/skills/judex/'));
  if (!found) throw new Error('Codex did not discover the installed Judex skill');
  evidence.codexDiscovery = true; evidence.codexSkill = {name: found.name, path: found.path, scope: found.scope};
  fs.writeFileSync(path.join(codexSkill, 'personal-note.txt'), 'preserve');
  invoke('skill', 'uninstall', '--target', 'path', '--path', codexSkill);
  if (fs.readFileSync(path.join(codexSkill, 'personal-note.txt'), 'utf8') !== 'preserve') throw new Error('Installer removed personal file');
  evidence.installerPreservesUserFiles = true;
  const after = await rpc('skills/list', {cwds: [root], forceReload: true});
  if (after.data.flatMap(entry => entry.skills ?? []).some(skill => skill.name === 'judex' && String(skill.path).includes(root))) throw new Error('Uninstalled skill still discovered');
  invoke('skill', 'uninstall', '--target', 'path', '--path', claudeSkill);
  evidence.uninstallVerified = true;
  console.log('Skill installer and Codex discovery passed:', root);
} finally {
  server.stdin.end(); server.kill();
  for (const waiter of pending.values()) clearTimeout(waiter.timer);
  fs.writeFileSync(path.join(root, 'manifest.json'), JSON.stringify(evidence, null, 2));
}
