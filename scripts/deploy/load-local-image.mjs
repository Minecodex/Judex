// Docker Desktop's Docker and Kubernetes containerd stores are separate.
// Keep the archive as local deployment evidence and stream its binary bytes.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
const image=process.argv[2];
if(!image||!/^\w[\w./:-]*:[\w.-]+$/.test(image))throw new Error('Pass a tagged local image');
const run=(name,args,options={})=>execFileSync(name,args,{encoding:'utf8',windowsHide:true,timeout:240000,...options});
if(run('kubectl',['config','current-context']).trim()!=='docker-desktop')throw new Error('Select the docker-desktop context before importing');
const inspected=JSON.parse(run('docker',['image','inspect',image]))[0];
if(inspected.Os!=='linux'||inspected.Architecture!=='amd64')throw new Error('The local Docker Desktop node requires a linux/amd64 image');
const archive=path.resolve(process.argv[3]??`.cache/deploy/image-${randomUUID()}.tar`);
if(fs.existsSync(archive))throw new Error('Image archive already exists; choose a fresh path');
fs.mkdirSync(path.dirname(archive),{recursive:true});
run('docker',['image','save','--platform','linux/amd64','-o',archive,image]);
run('docker',['exec','-i','desktop-control-plane','ctr','-n','k8s.io','images','import','--all-platforms','-'],{input:fs.readFileSync(archive),stdio:['pipe','inherit','inherit']});
const imported=run('docker',['exec','desktop-control-plane','ctr','-n','k8s.io','images','list','-q']).split(/\r?\n/);
const reference=image.includes('/')&&!image.split('/')[0].includes('.')&&!image.split('/')[0].includes(':')?'docker.io/'+image:image;
if(!imported.includes(reference))throw new Error('Imported image reference was not found in containerd');
console.log('Local Kubernetes image loaded:',image,'archive:',archive);
