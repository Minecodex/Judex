// Run against a disposable V2 cluster created by k8s.mjs.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {evidencePath} from './environment.mjs';
const image=process.env.JUDEX_CONVERTER_IMAGE;
if(!image)throw new Error('Set JUDEX_CONVERTER_IMAGE to the built converter image with LibreOffice, ffmpeg and libarchive.');
const directory=evidencePath('fixtures');fs.mkdirSync(directory,{recursive:true});
const run=(command,args)=>execFileSync(command,args,{stdio:'inherit',timeout:600000});
run('docker',['run','--rm','--user','0','--entrypoint','python3',
 '-v',path.resolve('tests/v2/readers-fixtures.py')+':/make-fixtures.py:ro',
 '-v',path.resolve('tests/fixtures/materials')+':/original:ro',
 '-v',directory+':/fixtures',image,'/make-fixtures.py']);
run(process.execPath,['tests/v2/readers-confirmations.mjs']);
if(process.env.JUDEX_V2_BROWSER==='1')run('npx',['playwright','test','--config','tests/v2/readers-browser.config.mjs']);
