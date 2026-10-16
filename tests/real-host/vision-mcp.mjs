// Official Coding Plan vision MCP, isolated from global client configuration.
// The user-provided key is read in memory from this run's owned Secret only.
import fs from 'node:fs';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
import {execFileSync} from 'node:child_process';
const [mode='list',toolName,inputPath]=process.argv.slice(2);
const root=path.resolve('.cache/real-host/20261012-closure');
const env=JSON.parse(fs.readFileSync(path.join(root,'environment.json'),'utf8'));
const run=args=>execFileSync('kubectl',args,{encoding:'utf8',windowsHide:true,maxBuffer:1024*1024});
const ns=JSON.parse(run(['get','namespace',env.namespace,'-o','json']));
if(ns.metadata.uid!==env.uid||ns.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Namespace ownership changed');
const secret=JSON.parse(run(['-n',env.namespace,'get','secret','real-model-key','-o','json']));
if(secret.metadata.labels?.['judex.dev/test-run']!==env.runId)throw new Error('Key Secret ownership changed');
const key=Buffer.from(secret.data.JUDEX_GLM_CODING_KEY,'base64').toString('utf8');
const modules=path.join(root,'vision-tool/node_modules');
const {Client}=await import(pathToFileURL(path.join(modules,'@modelcontextprotocol/sdk/dist/esm/client/index.js')));
const {StdioClientTransport}=await import(pathToFileURL(path.join(modules,'@modelcontextprotocol/sdk/dist/esm/client/stdio.js')));
const client=new Client({name:'judex-real-host-vision-preflight',version:'1.0.0'},{capabilities:{}});
const transport=new StdioClientTransport({command:process.execPath,args:[path.join(modules,'@z_ai/mcp-server/build/index.js')],env:{...process.env,Z_AI_API_KEY:key,Z_AI_MODE:'ZHIPU',Z_AI_VISION_MODEL:'glm-5.3-flash',Z_AI_VISION_MODEL_MAX_TOKENS:'4096',Z_AI_TIMEOUT:'120000'},stderr:'pipe'});
let safeError='';
transport.stderr?.on('data',chunk=>{safeError=(safeError+chunk.toString().split(key).join('[redacted]')).slice(-2000);});
try{
 await client.connect(transport,{timeout:30000});
 const listed=await client.listTools();
 if(mode==='list'){
  fs.writeFileSync(path.join(root,'vision-mcp-tools.json'),JSON.stringify({source:'official @z_ai/mcp-server',version:'0.1.5',mode:'ZHIPU',listedAt:new Date().toISOString(),tools:listed.tools},null,2)+'\n');
  console.log(JSON.stringify(listed.tools.map(t=>({name:t.name,inputSchema:t.inputSchema}))));
 }else if(mode==='call'){
  if(!listed.tools.some(t=>t.name===toolName))throw new Error('Observed official tool required');
  const input=JSON.parse(fs.readFileSync(inputPath,'utf8'));
  const started=Date.now();
  const result=await client.callTool({name:toolName,arguments:input},undefined,{timeout:180000});
  const observations=path.join(root,'vision-observations');fs.mkdirSync(observations,{recursive:true});
  for(const file of fs.readdirSync(observations)){
   const target=path.join(observations,file);
   if(/^vision-\d+\.json$/.test(file)&&fs.statSync(target).mtimeMs<Date.now()-86400000)fs.unlinkSync(target);
  }
  const safe=JSON.stringify(result).split(key).join('[redacted]');
  fs.writeFileSync(path.join(observations,`vision-${started}.json`),safe+'\n');
  console.log(JSON.stringify({tool:toolName,durationMs:Date.now()-started,isError:result.isError??false,result:JSON.parse(safe)}));
 }else throw new Error('Use list or call');
}catch(error){
 console.error(String(error.message).split(key).join('[redacted]'));
 if(safeError)console.error(safeError);
 process.exitCode=1;
}finally{await client.close();}
