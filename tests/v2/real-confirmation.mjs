import assert from 'node:assert/strict';
import {client} from './api.mjs';
import {readEnvironment,writeEvidence} from './environment.mjs';
if(process.env.JUDEX_V2_REAL_MODEL!=='1')throw new Error('Explicit live-model acceptance flag required');
const env=readEnvironment(),a=client(),b=client(),u=await a.register('真实确认 · 林舟'),v=await b.register('真实确认 · 陈柚');
const report={status:'running',version:env.version,provider:'APIMart',model:'glm-5.3-flash',checks:[]},save=()=>writeEvidence('real-confirmation.json',report);
async function wait(fn){const end=Date.now()+240000;while(Date.now()<end){const value=await fn();if(value)return value;await new Promise(r=>setTimeout(r,500));}throw new Error('Real confirmation deadline');}
try{
 const p=await a.call('POST','/projects',{name:'真实模型 · 确认卡片',defaultModelId:env.models.real,description:'仅使用合成方案验证人类确认工具，不执行外部操作。'});
 await a.call('POST',`/projects/${p.id}/members`,{userId:v,role:'member'});
 const position=await a.call('POST',`/projects/${p.id}/positions`,{name:'设计顾问',prompt:'收到请求后只调用一次 send_message：以 confirmation.title 标识事项，结构化提及指定的人且 requiresResponse=true。请其判断建议或提出修改。完成后结束，不调用其他工具，不执行任何外部动作。'});
 const g=await a.call('POST',`/projects/${p.id}/groups`,{name:'方案意见',purpose:'请人判断合成设计方案是否合适。',members:[{userId:u,positionIds:[position.id]},{userId:v,positionIds:[]}]});
 report.projectId=p.id;report.groupId=g.id;save();
 const role=g.members.find(m=>m.userId===u).roles[0].id;
 await a.call('POST',`/conversations/${g.id}/messages`,{body:`请通过 send_message 发布确认卡片，confirmation.title 为「品牌方案 B」，正文建议采用简洁的橄榄色布局并请求意见。mentions 只包括 kind=human、targetId=${v}、requiresResponse=true。发送后立即结束。`,mentions:[{kind:'role_agent',targetId:role}]});
 const card=await wait(async()=>(await a.call('GET',`/conversations/${g.id}/messages`)).items.find(m=>m.roleAgentId===role&&m.confirmation));
 assert.equal(card.confirmation.title,'品牌方案 B');assert.ok(card.mentions.some(m=>m.targetId===v&&m.requiresResponse));report.checks.push('real-send-message-confirmation-and-recipient');save();
 const d=await wait(async()=>{const ds=(await a.call('GET',`/groups/${g.id}/discussions`)).items,rs=(await a.call('GET',`/groups/${g.id}/runs`)).items;return ds.find(d=>d.waitingFor.length===1&&rs.every(r=>r.state==='completed'));});report.checks.push('real-role-waits-for-designated-human');
 const before=(await a.call('GET',`/groups/${g.id}/runs`)).items.length;
 await b.call('POST',`/conversations/${g.id}/messages`,{body:'需要修改，请先扩大正文行距。',replyTo:card.id,decision:{choice:'changes',expectedVersion:1}});
 await wait(async()=>(await a.call('GET',`/groups/${g.id}/discussions`)).items.find(x=>x.id===d.id&&x.state==='settled'));
 assert.equal((await a.call('GET',`/groups/${g.id}/runs`)).items.length,before);assert.equal((await a.call('GET',`/conversations/${g.id}/messages/${card.id}`)).confirmation.responses[0].choice,'changes');report.checks.push('human-choice-recorded-without-auto-model-call','real-waiting-branch-settles');
 report.runs=(await a.call('GET',`/groups/${g.id}/runs`)).items.map(r=>({id:r.id,state:r.state,modelCalls:r.modelCalls,inputTokens:r.inputTokens,outputTokens:r.outputTokens}));
 report.status='passed';save();console.log(JSON.stringify(report));
}catch(error){report.status='failed';report.error=String(error);save();throw error;}
