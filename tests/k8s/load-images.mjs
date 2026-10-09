// Import owned test images when Docker Desktop hides its node container from
// the Docker API. No production deployment, workload or runtime data is changed.
import {execFileSync} from "node:child_process";
import fs from "node:fs";
import path from "node:path";
const [namespace,runId,uid,...images]=process.argv.slice(2);
if(!namespace?.startsWith("judex-ui-")||namespace!==runId||!uid||!images.length)throw new Error("Pass an owned acceptance namespace, run id, uid and tagged test images");
const run=(name,args,input)=>execFileSync(name,args,{encoding:"utf8",input,windowsHide:true,timeout:120000,maxBuffer:16*1024*1024});
const kube=(...args)=>run("kubectl",args);
const ownership=()=>{const ns=JSON.parse(kube("get","namespace",namespace,"-o","json"));if(ns.metadata.uid!==uid||ns.metadata.labels?.["judex.dev/test-run"]!==runId)throw new Error("Namespace ownership mismatch");};
ownership();
const nodes=JSON.parse(kube("get","nodes","-o","json")).items;
if(nodes.length!==1||nodes[0].metadata.name!=="desktop-control-plane"||nodes[0].status.nodeInfo.architecture!=="amd64")throw new Error("This importer only supports the verified local desktop node");
const name="test-image-importer",labels={"judex.dev/test-run":runId};
run("kubectl",["apply","-f","-"],JSON.stringify({apiVersion:"v1",kind:"Pod",metadata:{name,namespace,labels},spec:{nodeName:"desktop-control-plane",automountServiceAccountToken:false,restartPolicy:"Never",containers:[{name:"importer",image:"debian:13-slim",imagePullPolicy:"IfNotPresent",command:["sleep","600"],securityContext:{runAsUser:0,allowPrivilegeEscalation:false,capabilities:{drop:["ALL"]}},volumeMounts:[{name:"ctr",mountPath:"/host/ctr",readOnly:true},{name:"socket",mountPath:"/run/containerd/containerd.sock"}]}],volumes:[{name:"ctr",hostPath:{path:"/usr/local/bin/ctr",type:"File"}},{name:"socket",hostPath:{path:"/run/containerd/containerd.sock",type:"Socket"}}]}}));
kube("-n",namespace,"wait","pod/"+name,"--for=condition=Ready","--timeout=60s");
for(const image of images){
 if(!/^(judex\/server|judex\/test-gateway|judex\/material-converter):collaboration-[\w.-]+$/.test(image))throw new Error("Only this collaboration run's images may be imported");
 ownership();
 const inspected=JSON.parse(run("docker",["image","inspect",image]))[0];
 if(inspected.Os!=="linux"||inspected.Architecture!=="amd64")throw new Error("Test image platform mismatch");
 const archive=path.resolve(".cache/k8s",runId,image.replace(/[/:]/g,"-")+".tar");
 fs.mkdirSync(path.dirname(archive),{recursive:true});
 run("docker",["image","save","--platform","linux/amd64","-o",archive,image]);
 run("kubectl",["-n",namespace,"exec","-i",name,"--","/host/ctr","-n","k8s.io","images","import","--all-platforms","-"],fs.readFileSync(archive));
 const refs=kube("-n",namespace,"exec",name,"--","/host/ctr","-n","k8s.io","images","list","-q").split(/\r?\n/);
 if(!refs.includes("docker.io/"+image))throw new Error("Imported test image not found");
 console.log("Verified test image available:",image);
}
ownership();kube("-n",namespace,"delete","pod",name,"--grace-period=1","--wait=true","--timeout=30s");
