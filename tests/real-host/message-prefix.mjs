// Local comparison of request messages, not a provider cache/tokenizer claim.
import {createHash} from 'node:crypto';

const ordered=value=>Array.isArray(value)?value.map(ordered):value&&typeof value==='object'?Object.fromEntries(Object.keys(value).sort().map(key=>[key,ordered(value[key])])):value;
const encode=value=>Buffer.from(JSON.stringify(ordered(value)));
const digest=bytes=>createHash('sha256').update(bytes).digest('hex');

export function prefixSnapshot(request){
 const messages=Array.isArray(request.messages)?request.messages:[];
 return {model:request.model,messages:messages.map(message=>({bytes:encode(message),hash:digest(encode(message))})),toolsHash:digest(encode(request.tools??[]))};
}

export function compareMessagePrefix(before,after){
 let completeMessages=0,commonBytes=0;
 for(let i=0;i<Math.min(before.messages.length,after.messages.length);i++){
  const a=before.messages[i],b=after.messages[i];
  if(a.hash===b.hash){completeMessages++;commonBytes+=a.bytes.length;continue;}
  for(let j=0;j<Math.min(a.bytes.length,b.bytes.length)&&a.bytes[j]===b.bytes[j];j++)commonBytes++;
  break;
 }
 return {sameModel:before.model===after.model,sameTools:before.toolsHash===after.toolsHash,completeMessages,commonBytes,beforeBytes:before.messages.reduce((sum,m)=>sum+m.bytes.length,0),afterBytes:after.messages.reduce((sum,m)=>sum+m.bytes.length,0)};
}

export function recordMessagePrefix(aggregate,previous,request){
 const current=prefixSnapshot(request);
 aggregate.requests++;
 if(previous){
  const result=compareMessagePrefix(previous,current);
  aggregate.comparisons++;
  aggregate.modelChanges+=Number(!result.sameModel);
  aggregate.toolChanges+=Number(!result.sameTools);
  aggregate.commonBytes+=result.commonBytes;
  aggregate.minCommonBytes=Math.min(aggregate.minCommonBytes??Infinity,result.commonBytes);
  aggregate.maxCommonBytes=Math.max(aggregate.maxCommonBytes??0,result.commonBytes);
  aggregate.completeMessageMatches+=result.completeMessages;
  if(result.commonBytes===result.beforeBytes)aggregate.previousMessagesRetained++;
 }
 return current;
}

export const newPrefixAggregate=()=>({scope:'local canonical message bytes; not upstream cache hits',requests:0,comparisons:0,modelChanges:0,toolChanges:0,commonBytes:0,minCommonBytes:null,maxCommonBytes:null,completeMessageMatches:0,previousMessagesRetained:0});
