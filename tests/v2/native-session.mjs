import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import assert from 'node:assert/strict';
import {spawn,execFileSync,execFile} from 'node:child_process';
import {base} from './api.mjs';

export async function withNativeCLI(who,projectIds,work,{baseURL=base,asyncCommands=false}={}){
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'judex-v2-native-'));
 const binary=path.resolve(process.env.JUDEX_V2_CLI||'.cache/judex-v2');
 const env={...process.env,JUDEX_CONFIG_DIR:directory};let stderr='';
 const login=spawn(binary,['auth','login','--server',baseURL,'--json'],{env});
 login.stderr.on('data',data=>stderr+=data);login.stdout.resume();
 const ended=new Promise((resolve,reject)=>{login.on('exit',resolve);login.on('error',reject);});
 try{
  const deadline=Date.now()+15000;let code;
  while(!(code=stderr.match(/代码：([^\s]+)/)?.[1])){if(Date.now()>deadline)throw new Error('Device code deadline');await new Promise(resolve=>setTimeout(resolve,100));}
  const review=await who.call('GET','/auth/device/review?userCode='+encodeURIComponent(code));
  await who.call('POST','/auth/device/confirm',{userCode:code,approved:true,scopes:review.scopes,projectScope:projectIds});
  assert.equal(await ended,0,'Native device authorization failed');
  const syncCLI=(...args)=>JSON.parse(execFileSync(binary,[...args,'--server',baseURL,'--json'],{env,encoding:'utf8'}));
  const cli=asyncCommands?(...args)=>new Promise((resolve,reject)=>execFile(binary,[...args,'--server',baseURL,'--json'],{env,encoding:'utf8'},(error,stdout,stderr)=>{if(error){error.stdout=stdout;error.stderr=stderr;reject(error);}else resolve(JSON.parse(stdout));})):syncCLI;
  return await work(cli,directory);
 }finally{login.kill();fs.rmSync(directory,{recursive:true,force:true});}
}
