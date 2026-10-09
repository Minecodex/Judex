// Opt-in model-backed host verification. Run only after explicit authorization.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {execFileSync, spawnSync} from 'node:child_process';
import {randomUUID} from 'node:crypto';
if (process.env.JUDEX_RUN_AGENT_HOST_TESTS !== '1') throw new Error('Explicit authorization is required for additional AI host sessions');
const artifact = path.resolve('.cache/skill-agent-hosts', randomUUID()); fs.mkdirSync(artifact, {recursive: true});
const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'judex-host-'));
const binaryName = process.platform === 'win32' ? 'judex.exe' : 'judex';
const binary = path.join(directory, binaryName);
execFileSync('go', ['build', '-o', binary, './cmd/judex'], {windowsHide: true, stdio: 'inherit'});
for (const folder of ['.agents/skills/judex', '.claude/skills/judex']) execFileSync(binary, ['--json', 'skill', 'install', '--target', 'path', '--path', path.join(directory, folder)], {windowsHide: true, stdio: 'pipe'});
fs.writeFileSync(path.join(directory, 'AGENTS.md'), 'This is an isolated Judex CLI acceptance fixture. Read only the installed judex skill and its references. Run only the local CLI version/help commands. Do not access servers, log in, upload, approve, edit files, use other agents, or read other directories.\n');
const common = `我现在需要使用 Judex。请按已安装的 judex 技能先检查客户端能力。本次范围仅限当前目录：读取技能及命令参考，运行 ./${binaryName} --json version 和 ./${binaryName} --help。先不访问任何服务器，不登录、不上传、不作正式决定，不修改文件、不使用其他代理。最后报告实际客户端版本、技能名称和人工确认边界；命令失败则如实说明。`;
const results = [];
for (const host of ['codex', 'claude']) {
  const command = host === 'codex' && process.env.JUDEX_CODEX_ENTRY ? process.execPath : host;
  const args = host === 'codex'
    ? [...(process.env.JUDEX_CODEX_ENTRY ? [process.env.JUDEX_CODEX_ENTRY] : []), '-a', 'never', 'exec', '--ephemeral', '--skip-git-repo-check', '--sandbox', 'read-only', '--json', '-C', directory, '$judex ' + common]
    : ['--print', '--no-session-persistence', '--output-format', 'stream-json', '--verbose', '--permission-mode', 'dontAsk', '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}', '--settings', '{"disableAllHooks":true}', '--tools', 'Read,Bash,Skill', '--allowedTools', `Read,Skill,Bash(./${binaryName} --json version),Bash(./${binaryName} --help)`, '/judex ' + common];
  const result = spawnSync(command, args, {cwd: directory, env: {...process.env, JUDEX_CONFIG_DIR: path.join(directory, 'credentials'), JUDEX_TOKEN: ''}, windowsHide: true, encoding: 'utf8', timeout: 180000, maxBuffer: 8 << 20});
  fs.writeFileSync(path.join(artifact, host + '.jsonl'), result.stdout ?? '');
  fs.writeFileSync(path.join(artifact, host + '.stderr.log'), result.stderr ?? '');
  results.push({host, exitCode: result.status, processError: result.error?.message ?? null, requiresTraceReview: true});
}
fs.writeFileSync(path.join(artifact, 'manifest.json'), JSON.stringify({directory, results, scope: 'version/help only; no Judex server or business operations'}, null, 2));
console.log('Host traces require review:', artifact);
