import fs from 'node:fs';
import path from 'node:path';

const settings=typeof process==='undefined'?{}:process.env;
export const evidenceDirectory=path.resolve(settings.JUDEX_V2_EVIDENCE||'.cache/v2-k8s');
export const evidencePath=name=>path.join(evidenceDirectory,name);
export const readEnvironment=()=>JSON.parse(fs.readFileSync(settings.JUDEX_V2_ENV||evidencePath('environment.json'),'utf8'));
export function writeEvidence(name,value){fs.mkdirSync(evidenceDirectory,{recursive:true});fs.writeFileSync(evidencePath(name),JSON.stringify(value,null,2)+'\n');}
