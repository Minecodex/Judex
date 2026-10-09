import fs from 'node:fs';
import path from 'node:path';
import { createHash, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';

const directory=path.resolve(process.argv[2]??'web/dist/downloads');
const manifest=JSON.parse(fs.readFileSync(path.join(directory,'manifest.json'),'utf8'));
const output=path.resolve('.cache/download-tests',randomUUID());fs.mkdirSync(output,{recursive:true});
for(const [platform,artifact]of Object.entries(manifest.artifacts)){
  const file=path.join(directory,artifact.filename),bytes=fs.readFileSync(file);
  if(bytes.length!==artifact.size||createHash('sha256').update(bytes).digest('hex')!==artifact.sha256)throw new Error('Artifact verification failed: '+platform);
  const inspect=`import sys,zipfile,struct,json,pathlib\nz=zipfile.ZipFile(sys.argv[1]); platform=sys.argv[2]; version=sys.argv[3]\nfor i in z.infolist():\n assert not i.filename.startswith('/') and '..' not in pathlib.PurePosixPath(i.filename).parts\n if i.filename.endswith('manifest.json') and 'judex' in i.filename: assert json.loads(z.read(i))['version']==version\nif platform=='windows':\n assert 'judex.cmd' in z.namelist()\n for arch,machine in [('amd64',0x8664),('arm64',0xaa64)]:\n  data=z.read('windows-'+arch+'/judex.exe'); assert data[:2]==b'MZ'; pe=struct.unpack_from('<I',data,0x3c)[0]; assert data[pe:pe+4]==b'PE\\x00\\x00'; assert struct.unpack_from('<H',data,pe+4)[0]==machine\nelif platform=='macos':\n assert 'judex' in z.namelist()\n for arch,cpu in [('amd64',0x01000007),('arm64',0x0100000c)]:\n  name='macos-'+arch+'/judex'; data=z.read(name); assert struct.unpack_from('<I',data,0)[0]==0xfeedfacf; assert struct.unpack_from('<I',data,4)[0]==cpu; assert (z.getinfo(name).external_attr>>16)&0o111\n assert (z.getinfo('judex').external_attr>>16)&0o111\nelse:\n assert 'judex/SKILL.md' in z.namelist() and 'judex/references/commands.md' in z.namelist()\nz.extractall(sys.argv[4])\nprint(platform+' archive verified')`;
  console.log(execFileSync(process.env.JUDEX_PYTHON??'python',['-c',inspect,file,platform,manifest.version,path.join(output,platform)],{encoding:'utf8',windowsHide:true}).trim());
}
if(process.platform==='win32'){
  const cli=path.join(output,'windows',`windows-${process.arch==='arm64'?'arm64':'amd64'}`,'judex.exe');
  const invoke=(...args)=>JSON.parse(execFileSync(cli,['--json',...args],{encoding:'utf8',windowsHide:true}));
  const version=invoke('version');if(version.data?.cli!==manifest.version||!version.ok)throw new Error('Native CLI version mismatch');
  const launcher=path.join(output,'windows','judex.cmd');
  const launched=JSON.parse(execFileSync(process.env.ComSpec??'cmd.exe',['/d','/s','/c',`""${launcher}" --json version"`],{encoding:'utf8',windowsHide:true,windowsVerbatimArguments:true}));
  if(!launched.ok||launched.data?.cli!==manifest.version)throw new Error('Windows launcher version mismatch');
  const installed=path.join(output,'installed','judex');invoke('skill','install','--target','path','--path',installed);
  if(JSON.parse(fs.readFileSync(path.join(installed,'manifest.json'),'utf8')).version!==manifest.version)throw new Error('Installed skill version mismatch');
  fs.writeFileSync(path.join(installed,'user-note.txt'),'preserve');invoke('skill','uninstall','--target','path','--path',installed);
  if(fs.existsSync(path.join(installed,'SKILL.md'))||fs.readFileSync(path.join(installed,'user-note.txt'),'utf8')!=='preserve')throw new Error('Skill installer ownership failed');
}
fs.writeFileSync(path.join(output,'manifest.json'),JSON.stringify({version:manifest.version,archiveChecksumsVerified:true,windowsArchitecturesVerified:true,macosArchitecturesAndPermissionsVerified:true,skillsVerified:true,nativeCLIAndInstallerVerified:process.platform==='win32',nativeWindowsLauncherVerified:process.platform==='win32'},null,2));
console.log('Download verification passed:',output);
