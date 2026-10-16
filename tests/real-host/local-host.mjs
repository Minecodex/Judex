import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const version=process.argv[2];
if(!/^[0-9A-Za-z][0-9A-Za-z.+-]*$/.test(version??''))throw new Error('Pass the release version');
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const candidate=path.resolve('dist/release',version,'cli/windows-amd64');
const scenes={'A-MiniIssue':['KA01','KA02','KA03','KA04','KA05','KA06','KX01','KX02'],'B-VisualAssets':['KB01','KB02','KB03'],'C-OfficeReports':['KC01','KC02','KC03']};
for(const [folder,sessions] of Object.entries(scenes)){
 const root=path.join(env.localRoot,folder),tools=path.join(root,'.tools');
 fs.mkdirSync(path.join(tools,'skills'),{recursive:true});
 fs.copyFileSync(path.join(candidate,'judex.exe'),path.join(tools,'judex.exe'));
 fs.cpSync(path.join(candidate,'skills/judex'),path.join(tools,'skills/judex'),{recursive:true});
 execFileSync(path.join(tools,'judex.exe'),['--json','skill','install','--target','path','--path',path.join(root,'.agents/skills/judex')],{windowsHide:true,stdio:'pipe'});
 for(const session of sessions){
  const config=path.join(tools,'config',session);fs.mkdirSync(config,{recursive:true});
  if(!fs.existsSync(path.join(config,'project.json')))fs.writeFileSync(path.join(config,'project.json'),JSON.stringify({server:`http://127.0.0.1:${env.port}`},null,2));
  fs.writeFileSync(path.join(tools,session+'.cmd'),`@echo off\r\nset "JUDEX_CONFIG_DIR=%~dp0config\\${session}"\r\n"%~dp0judex.exe" --profile ${session} %*\r\n`);
 }
 const ignorePath=path.join(root,'.gitignore');
 const existingIgnore=fs.existsSync(ignorePath)?fs.readFileSync(ignorePath,'utf8'):'';
 const rules=['.tools/','.judex/','.agents/','__pycache__/','*.db','*.sqlite'];
 const missing=rules.filter(rule=>!existingIgnore.split(/\r?\n/).includes(rule));
 if(missing.length)fs.writeFileSync(ignorePath,existingIgnore+(existingIgnore&&!existingIgnore.endsWith('\n')?'\n':'')+missing.join('\n')+'\n');
 fs.writeFileSync(path.join(root,'test-connection.json'),JSON.stringify({server:`http://127.0.0.1:${env.port}`,version,models:env.models,sessionCommands:sessions.map(s=>`.tools/${s}.cmd`),scope:'Only this isolated acceptance environment; read current formal business state through the public CLI'},null,2)+'\n');
 console.log(folder+' prepared with packaged CLI/Skill '+version);
}
