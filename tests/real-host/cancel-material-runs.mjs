// Manager execution control through the supported native CLI. No SQL mutations.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const root='.cache/real-host/20261012-closure';
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const namespace=JSON.parse(execFileSync('kubectl',['get','namespace',env.namespace,'-o','json'],{encoding:'utf8',windowsHide:true}));
if(namespace.metadata.uid!==env.uid||namespace.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Namespace ownership changed');
const project='21a13532-7929-4c6c-99a7-01e044eee64e',local=path.join(env.localRoot,'A-MiniIssue');
const rows=JSON.parse(fs.readFileSync(path.join(root,'redundant-material-analysis-runs.json'),'utf8'));
const invoke=args=>{
 try{return JSON.parse(execFileSync(path.join(local,'.tools/judex.exe'),['--profile','KA01','--json','--project',project,...args],{cwd:local,encoding:'utf8',windowsHide:true,env:{...process.env,JUDEX_CONFIG_DIR:path.join(local,'.tools/config/KA01'),JUDEX_TOKEN:''}}));}
 catch(error){if(error.stdout)return JSON.parse(error.stdout);throw error;}
};
const log=[];
for(const row of rows){
 if(row.purpose!=='material'||!/^[0-9a-f-]{36}$/.test(row.runId)||!['f3309487-a97d-4880-95ab-6eb1d290e69c','3309af7c-36cf-4b38-8292-a3d3b232fdf7'].includes(row.taskId))throw new Error('Only identified A12/A13 material-upload analyses may be controlled');
 const current=invoke(['run','show',row.runId]);
 if(!current.ok)throw new Error('Named manager could not read its own scene run');
 if(!['running','queued','waiting_human'].includes(current.data.state)){log.push({runId:row.runId,state:current.data.state,action:'already_terminal'});continue;}
 const cancelled=invoke(['run','cancel',row.runId,'--reason','这些原件已纳入完整交付报告，停止逐文件重复分析；保留全部原件/来源和失败证据，后续只分析完整交付报告']);
 if(!cancelled.ok)throw new Error('Supported manager CLI cancellation failed: '+cancelled.error);
 log.push({runId:row.runId,sourceId:row.sourceId,taskId:row.taskId,action:'cancelled_by_U0_manager_cli',at:new Date().toISOString()});
 fs.writeFileSync(path.join(root,'material-run-cancellation-log.json'),JSON.stringify(log,null,2)+'\n');
}
console.log(JSON.stringify({controlledRuns:log.length,cancelled:log.filter(r=>r.action.startsWith('cancelled')).length,formalTasksChanged:false,originalsPreserved:true}));
