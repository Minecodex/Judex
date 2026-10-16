// Standard packaged CLI device flow, approved by the named test user's UI.
import fs from 'node:fs';
import path from 'node:path';
import {spawn,execFileSync} from 'node:child_process';
const [label,actor,requestedScopes]=process.argv.slice(2);
const folder=/^KA|^KX/.test(label??'')?'A-MiniIssue':/^KB/.test(label??'')?'B-VisualAssets':/^KC/.test(label??'')?'C-OfficeReports':null;
if(!folder||!/^U[0-5]$/.test(actor??''))throw new Error('Explicit scene label and test actor required');
const env=JSON.parse(fs.readFileSync('.cache/real-host/20261012-closure/environment.json','utf8'));
const cwd=path.join(env.localRoot,folder),config=path.join(cwd,'.tools/config',label);
const args=['--profile',label,'--server',`http://127.0.0.1:${env.port}`,'--json','auth','login'];
if(requestedScopes){if(!/^[a-z:,]+$/.test(requestedScopes))throw new Error('Explicit public scope names required');args.push('--scopes',requestedScopes);}
const child=spawn(path.join(cwd,'.tools/judex.exe'),args,{cwd,windowsHide:true,env:{...process.env,JUDEX_CONFIG_DIR:config,JUDEX_TOKEN:''},stdio:['ignore','pipe','pipe']});
let output='',errors='',approved=false,approvalError;
child.stdout.on('data',chunk=>output+=chunk);
child.stderr.on('data',chunk=>{
 errors+=chunk;
 const match=errors.match(/输入代码[：:]\s*([A-Za-z0-9_-]{4,24})/);
 if(match&&!approved){
  approved=true;
  try{const result=execFileSync(process.execPath,['tests/real-host/ui-driver.mjs','device',actor,match[1]],{windowsHide:true,encoding:'utf8',timeout:45000});console.log(result.trim());}
  catch(error){approvalError=error.message;child.kill();}
 }
});
const code=await new Promise((resolve,reject)=>{child.on('error',reject);child.on('close',code=>resolve(code));});
if(code!==0||approvalError)throw new Error('CLI device authorization failed: '+(approvalError??output));
const result=JSON.parse(output.trim());if(!result.ok)throw new Error('CLI did not finish authorization');
console.log(label+' packaged CLI login completed for '+actor+' with browser-selected project access');
