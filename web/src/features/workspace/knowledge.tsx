import {useQuery} from '@tanstack/react-query';
import {useState} from 'react';
import {useWorkspace} from './context';
import {Button,Choice,Dialog,Empty,Field,QueryState,Tabs} from './ui';
import {list} from './api';
import type {Knowledge,Run,Snapshot} from './types';
import {MarkdownReader} from '../../components/readers/MarkdownReader';

type ToolCall={callId:string;name:string;args:unknown;state:string;result:unknown;replayAuthorized:boolean;version:number};
function EffectReview({call,runId,onSaved}:{call:ToolCall;runId:string;onSaved:()=>void}){
 const w=useWorkspace(),[resolution,setResolution]=useState('completed'),[reason,setReason]=useState(''),[result,setResult]=useState('');
 const save=async()=>{try{await w.execute(`/groups/${w.groupId}/runs/${runId}/tools/${encodeURIComponent(call.callId)}/reconcile`,{expectedVersion:call.version,resolution,reason,result:result?JSON.parse(result):{verified:true}});onSaved();}catch(error){w.setToast((error as Error).message);}};
 return <section className="judex-v2-template-editor"><strong>{call.name} · {call.state}</strong><pre className="judex-v2-code">{JSON.stringify(call.args,null,2)}</pre>{call.replayAuthorized?<p>{w.t('v2RetryAuthorized')}</p>:<><Choice label={w.t('v2EffectResolution')} value={resolution} onChange={setResolution} items={[{id:'completed',name:w.t('v2EffectCompleted')},{id:'retry',name:w.t('v2EffectRetry')}]}/><Field label={w.t('v2EffectReason')} value={reason} onChange={setReason} multiline/>{resolution==='completed'&&<Field label={w.t('v2EffectResult')} value={result} onChange={setResult} multiline/>}<Button variant="primary" isDisabled={!reason.trim()||w.pending} onPress={()=>void save()}>{w.t('v2EffectSave')}</Button></>}</section>;
}
function RunEntry({run}:{run:Run}){
 const w=useWorkspace(),[show,setShow]=useState(false),person=w.group?.members.find(m=>m.roles.some(r=>r.id===run.roleAgentId)),role=person?.roles.find(r=>r.id===run.roleAgentId);
 const tools=useQuery({queryKey:['v2',w.user.id,'own-tools',run.id],queryFn:()=>list<ToolCall>(`/groups/${w.groupId}/runs/${run.id}/tools`),enabled:show&&person?.userId===w.user.id,retry:false});
 return <section className="judex-v2-template-editor"><strong>{person?.displayName} · {role?.name} · {run.state}</strong><p>{w.t('vpSnapshot')} v{run.snapshotVersion??'—'} · {run.modelCalls} calls · {run.tokens} tokens</p>{run.errorMessage&&<p className="judex-v2-error">{run.errorMessage}</p>}{person?.userId===w.user.id&&<><Button onPress={()=>setShow(!show)}>{w.t('v2OwnJournal')}</Button>{show&&<><p>{w.t('v2EffectHint')}</p>{tools.isError?<Empty title={w.t('v2Failed')}/>:tools.data?.map(call=>(call.state==='started'||call.state==='unknown')&&run.state==='paused'?<EffectReview key={call.callId+':'+call.version} call={call} runId={run.id} onSaved={()=>void tools.refetch()}/>:<details key={call.callId}><summary>{call.name} · {call.state}</summary><pre className="judex-v2-code">{JSON.stringify({args:call.args,result:call.result},null,2)}</pre></details>)}</>}</>}</section>;
}
export function KnowledgePanel(){
 const w=useWorkspace(),[tab,setTab]=useState('summary'),[after,setAfter]=useState(0);
 const entries=useQuery({queryKey:['v2',w.user.id,'knowledge',w.groupId,after],queryFn:()=>list<Knowledge>(`/groups/${w.groupId}/context?after=${after}&limit=50`)});
 const snapshots=useQuery({queryKey:['v2',w.user.id,'snapshots',w.groupId],queryFn:()=>list<Snapshot>(`/groups/${w.groupId}/snapshots`)});
 return <Dialog wide title={w.t('vpKnowledge')} description={w.t('vpSharedContextHint')}><div className="judex-v2-form-stack">
  <Tabs label={w.t('vpKnowledge')} value={tab} onChange={setTab} items={[{id:'summary',label:w.t('vpSummary')},{id:'original',label:w.t('vpChatHistory')},{id:'runs',label:w.t('v2Running')}]}/>
  {tab==='summary'?<QueryState query={snapshots} empty={!snapshots.data?.length} emptyTitle={w.t('v2NoSummary')}>{snapshots.data?.map(s=><section className="judex-v2-template-editor" key={s.version}><strong>{w.t('vpSnapshot')} v{s.version} · {s.estimatedTokens} tokens</strong><p>{w.t('v2Covered')} {s.coveredThrough}</p><MarkdownReader text={s.summary||w.t('v2NoSummary')}/></section>)}</QueryState>
   :tab==='runs'?<QueryState query={w.runs} empty={!w.runs.data?.length}>{w.runs.data?.map(run=><RunEntry key={run.id} run={run}/>)}</QueryState>
   :<QueryState query={entries} empty={!entries.data?.length}>{entries.data?.map(entry=><details className="judex-v2-template-editor" key={entry.seq}><summary>#{entry.seq} · {entry.kind} · {entry.refId}</summary><pre className="judex-v2-code">{JSON.stringify(entry.content,null,2)}</pre></details>)}<div className="judex-v2-edit-row"><Button isDisabled={after===0} onPress={()=>setAfter(0)}>{w.t('vpBack')}</Button><Button isDisabled={(entries.data?.length||0)<50} onPress={()=>setAfter(entries.data?.at(-1)?.seq||0)}>{w.t('matNext')}</Button></div></QueryState>}
 </div></Dialog>;
}
