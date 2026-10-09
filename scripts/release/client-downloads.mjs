// Client artifacts are built from this source tree and served by the web image.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root=fileURLToPath(new URL('../../',import.meta.url));
const run=(command,args,options={})=>execFileSync(command,args,{cwd:root,windowsHide:true,...options});
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const skillFiles=['SKILL.md','manifest.json','references/commands.md','references/reporting.md','references/human-decisions.md'];

function fingerprint(version,commit) {
  const records=[];
  const visit=p=>{const info=fs.lstatSync(p);if(info.isSymbolicLink())throw new Error('Client input symlink: '+p);if(info.isDirectory())for(const name of fs.readdirSync(p).sort())visit(path.join(p,name));else records.push(path.relative(root,p).replaceAll('\\','/')+'\0'+sha(fs.readFileSync(p)));};
  for(const input of ['cmd/judex','internal','pkg','skills/judex','go.mod','go.sum','scripts/release/client-downloads.mjs'])visit(path.join(root,input));
  return sha(Buffer.from(JSON.stringify({version,commit,records})));
}
function copySkill(destination,version) {
  fs.mkdirSync(destination,{recursive:true});
  for(const name of skillFiles){const target=path.join(destination,name);fs.mkdirSync(path.dirname(target),{recursive:true});fs.copyFileSync(path.join(root,'skills/judex',name),target);}
  const manifest=JSON.parse(fs.readFileSync(path.join(destination,'manifest.json'),'utf8'));manifest.version=version;
  fs.writeFileSync(path.join(destination,'manifest.json'),JSON.stringify(manifest,null,2)+'\n');
}
function archive(source,target) {
  const code='import sys,pathlib,zipfile,stat\nroot=pathlib.Path(sys.argv[1])\nwith zipfile.ZipFile(sys.argv[2],"w",zipfile.ZIP_DEFLATED,compresslevel=9) as z:\n for p in sorted(root.rglob("*")):\n  if not p.is_file(): continue\n  name=p.relative_to(root).as_posix()\n  i=zipfile.ZipInfo(name,(1980,1,1,0,0,0)); i.create_system=3; i.compress_type=zipfile.ZIP_DEFLATED\n  mode=0o755 if p.name=="judex" or p.suffix==".exe" else 0o644\n  i.external_attr=(stat.S_IFREG|mode)<<16\n  z.writestr(i,p.read_bytes(),compress_type=zipfile.ZIP_DEFLATED,compresslevel=9)';
  run(process.env.JUDEX_PYTHON??'python',['-c',code,source,target]);
}

export function buildClientDownloads({version='0.1.0-dev',commit,output=path.join(root,'web/dist/downloads')}={}) {
  if(!/^[0-9A-Za-z][0-9A-Za-z.+-]*$/.test(version))throw new Error('Invalid client release version');
  commit??=String(run('git',['rev-parse','--short','HEAD'],{encoding:'utf8'})).trim()+(String(run('git',['status','--porcelain'],{encoding:'utf8'})).trim()?'-dirty':'');
  const sourceHash=fingerprint(version,commit);
  const cache=path.join(root,'.cache/client-downloads',version+'-'+sourceHash.slice(0,16));
  fs.mkdirSync(cache,{recursive:true});
  const catalogPath=path.join(cache,'manifest.json');
  let catalog;
  if(fs.existsSync(catalogPath)){
    const previous=JSON.parse(fs.readFileSync(catalogPath,'utf8'));
    if(Object.values(previous.artifacts).every(a=>fs.existsSync(path.join(cache,a.filename))&&sha(fs.readFileSync(path.join(cache,a.filename)))===a.sha256))catalog=previous;
  }
  if(!catalog){
    const windows=path.join(cache,'windows'),macos=path.join(cache,'macos'),skill=path.join(cache,'skill');
    for(const [goos,arch]of [['windows','amd64'],['windows','arm64'],['darwin','amd64'],['darwin','arm64']]){
      const platform=goos==='darwin'?'macos':'windows';
      const directory=path.join(cache,platform,`${platform}-${arch}`);fs.mkdirSync(directory,{recursive:true});
      const executable=path.join(directory,goos==='windows'?'judex.exe':'judex');
      run('go',['build','-trimpath','-ldflags',`-s -w -X github.com/kakj-go/Judex/internal/version.Version=${version} -X github.com/kakj-go/Judex/internal/version.Commit=${commit}`,'-o',executable,'./cmd/judex'],{env:{...process.env,GOOS:goos,GOARCH:arch,CGO_ENABLED:'0'},stdio:'inherit'});
      copySkill(path.join(directory,'skills/judex'),version);
      console.log('Built downloadable CLI',platform,arch);
    }
    fs.writeFileSync(path.join(windows,'judex.cmd'),'@echo off\r\nif /I "%PROCESSOR_ARCHITECTURE%"=="ARM64" goto arm64\r\nif /I "%PROCESSOR_ARCHITEW6432%"=="ARM64" goto arm64\r\n"%~dp0windows-amd64\\judex.exe" %*\r\nexit /b %ERRORLEVEL%\r\n:arm64\r\n"%~dp0windows-arm64\\judex.exe" %*\r\nexit /b %ERRORLEVEL%\r\n');
    fs.writeFileSync(path.join(macos,'judex'),'#!/bin/sh\nset -eu\nROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)\ncase "$(uname -m)" in\n  arm64|aarch64) exec "$ROOT/macos-arm64/judex" "$@" ;;\n  x86_64|amd64) exec "$ROOT/macos-amd64/judex" "$@" ;;\n  *) echo "Unsupported macOS architecture" >&2; exit 1 ;;\nesac\n');
    const instructions=`Judex ${version}\n\nWindows: extract and run .\\judex.cmd version\nmacOS: extract and run ./judex version (Intel and Apple Silicon are selected automatically).\nIf an extraction tool drops execute permissions: chmod +x judex macos-*/judex\n\nConnect: judex --server YOUR_TEAM_URL auth login\nInstall the bundled skill: judex skill install --target codex\nOr: judex skill install --target claude-code\n\nThe CLI works locally. Business approval is confirmed in the browser.\n`;
    for(const directory of [windows,macos]){fs.writeFileSync(path.join(directory,'README.txt'),instructions);for(const name of ['LICENSE','NOTICE'])fs.copyFileSync(path.join(root,name),path.join(directory,name));}
    copySkill(path.join(skill,'judex'),version);
    fs.writeFileSync(path.join(skill,'README.txt'),'Copy the judex folder into your host skill directory, or install the matching CLI and run judex skill install.\n');
    catalog={version,commit,sourceHash,artifacts:{}};
    for(const [key,directory]of [['windows',windows],['macos',macos],['skills',skill]]){
      const filename=`judex-${key}-${version}.zip`,target=path.join(cache,filename);archive(directory,target);
      const bytes=fs.readFileSync(target);catalog.artifacts[key]={filename,url:'/downloads/'+filename,size:bytes.length,sha256:sha(bytes),architectures:key==='skills'?[]:['amd64','arm64']};
    }
    if(fingerprint(version,commit)!==sourceHash)throw new Error('Client source changed during packaging');
    fs.writeFileSync(catalogPath,JSON.stringify(catalog,null,2)+'\n');
  }
  fs.mkdirSync(output,{recursive:true});
  for(const artifact of Object.values(catalog.artifacts))fs.copyFileSync(path.join(cache,artifact.filename),path.join(output,artifact.filename));
  fs.writeFileSync(path.join(output,'manifest.json'),JSON.stringify(catalog,null,2)+'\n');
  console.log('Client downloads ready:',version,path.relative(root,output));return catalog;
}
if(process.argv[1]&&import.meta.url===pathToFileURL(path.resolve(process.argv[1])).href){
  buildClientDownloads({version:process.env.JUDEX_RELEASE_VERSION??'0.1.0-dev',output:path.resolve(process.cwd(),process.argv[2]??'web/dist/downloads')});
}
