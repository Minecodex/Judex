// Run only hash-matching actually delivered software with a separate test DB.
import fs from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {spawn,execFileSync} from 'node:child_process';
import http from 'node:http';
const root=path.resolve('.cache/real-host/20261012-closure');
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const local=path.join(env.localRoot,'A-MiniIssue'),runtime=path.join(root,'miniissue-runtime'),source=path.join(runtime,'source');
const rework=process.argv.includes('--rework');
const themeRework=process.argv.includes('--theme-rework');
const patchA22=process.argv.includes('--patch-a22');
if(themeRework&&!rework)throw new Error('Theme revision must use the accepted backend revision');
if(patchA22&&!themeRework)throw new Error('A22 regression must retain the accepted theme revision');
const deliveredVersions=[];
fs.mkdirSync(source,{recursive:true});
for(const name of ['KA02-A12-delivery.json','KA03-A13-first-delivery.json']){
 if(rework&&name.startsWith('KA02'))continue;
 if(themeRework&&name.startsWith('KA03'))continue;
 const receipt=JSON.parse(fs.readFileSync(path.join(local,'receipts',name),'utf8'));
 const materials=receipt.materials??receipt.materialVersions;
 if(!Array.isArray(materials))throw new Error('Actual delivery receipt material list required');
 for(const item of materials){
  const file=item.file??item.path;
  if(!/^(backend|web)\//.test(file??''))continue;
  const original=path.resolve(local,file),destination=path.resolve(source,file);
  if(!original.startsWith(path.resolve(local)+path.sep)||!destination.startsWith(source+path.sep))throw new Error('Delivered path escapes the scene');
  const bytes=fs.readFileSync(original);
  if(createHash('sha256').update(bytes).digest('hex')!==item.sha256)throw new Error('Local file differs from the immutable delivered original '+file);
  fs.mkdirSync(path.dirname(destination),{recursive:true});fs.writeFileSync(destination,bytes);
 }
}
function copyDeliveredArchive(receiptName,prefix,expectedCount){
 const receipt=JSON.parse(fs.readFileSync(path.join(local,'receipts',receiptName),'utf8'));
 const item=receipt.materials.find(m=>m.file.endsWith('.zip'));
 const archive=path.resolve(local,item.file);
 if(!archive.startsWith(path.resolve(local)+path.sep)||createHash('sha256').update(fs.readFileSync(archive)).digest('hex')!==item.sha256)throw new Error('Rework ZIP differs from uploaded original');
 execFileSync('C:/Users/23574/AppData/Local/Programs/Python/Python312/python.exe',['-c',
  'import pathlib,sys,zipfile; root=pathlib.Path(sys.argv[2]).resolve(); z=zipfile.ZipFile(sys.argv[1]); entries=[e for e in z.infolist() if e.filename.startswith(sys.argv[3]+"/") and not e.is_dir()]; assert len(entries)==int(sys.argv[4]);\nfor e in entries:\n p=root/e.filename; assert not pathlib.PurePosixPath(e.filename).is_absolute() and ".." not in pathlib.PurePosixPath(e.filename).parts and ":" not in e.filename and "\\\\" not in e.filename and (e.external_attr >> 16) & 0o170000 != 0o120000 and e.file_size <= 1048576 and p.resolve().is_relative_to(root); p.parent.mkdir(parents=True,exist_ok=True); p.write_bytes(z.read(e))',archive,source,prefix,String(expectedCount)],{windowsHide:true,stdio:'pipe'});
 deliveredVersions.push({versionId:item.versionId,sha256:item.sha256,prefix});
}
if(rework)copyDeliveredArchive('KA02-A12-rework-delivery.json','backend',5);
if(themeRework)copyDeliveredArchive('KA03-A13-theme-rework-delivery.json','web',4);
if(patchA22)copyDeliveredArchive('KA02-A22-delivery.json','backend',5);
const port=18481,db=path.join(runtime,'miniissue-test.db');
const child=spawn('C:/Users/23574/AppData/Local/Programs/Python/Python312/python.exe',['-m','backend.server','--db',db,'--port',String(port),'--web-root',path.join(source,'web')],{cwd:source,windowsHide:true,env:{...process.env,PYTHONIOENCODING:'utf-8'},stdio:['ignore','pipe','pipe']});
const log=fs.createWriteStream(path.join(runtime,'runtime.log'),{flags:'a'});child.stdout.pipe(log);child.stderr.pipe(log);
fs.writeFileSync(path.join(runtime,'process.json'),JSON.stringify({parentPid:process.pid,pid:child.pid,port,db,source,namespace:env.namespace,uid:env.uid,deliveredVersions,startedAt:new Date().toISOString()},null,2)+'\n');
let ready=false;
const health=()=>new Promise(resolve=>{
 const request=http.get({hostname:'127.0.0.1',port,path:'/api/health',agent:new http.Agent(),timeout:1000},response=>{response.resume();resolve(response.statusCode===200);});
 request.on('error',()=>resolve(false));request.on('timeout',()=>{request.destroy();resolve(false);});
});
for(let attempt=0;attempt<30;attempt++){
 if(await health()){ready=true;break;}
 await new Promise(resolve=>setTimeout(resolve,250));
}
if(!ready){child.kill();throw new Error('Delivered backend did not become healthy');}
console.log(`Hash-verified delivered MiniIssue ready http://127.0.0.1:${port}`);
const heartbeat=setInterval(()=>console.log('MiniIssue runtime healthy; actual QA may use the isolated database'),45000);
for(const signal of ['SIGINT','SIGTERM'])process.on(signal,()=>child.kill());
child.on('exit',code=>{clearInterval(heartbeat);log.end();process.exitCode=code??1;});
