import {useState} from 'react';
import {Check,ChevronDown,ClipboardCheck,Pencil,Send} from 'lucide-react';
import {useWorkspace} from './context';
import {ActionMenu,Button,Field,CheckField} from './ui';
import {FileCard} from './Files';
import {MarkdownReader} from '../../components/readers/MarkdownReader';
import type {Message,ConfirmationResponse} from './types';
import '../../styles/workspace-confirmation.css';
type Choice=ConfirmationResponse['choice'];
export function ConfirmationCard({message:m}:{message:Message}){
 const w=useWorkspace(),[editing,setEditing]=useState(false),[feedback,setFeedback]=useState(''),[choice,setChoice]=useState<Choice>('agree'),[wake,setWake]=useState(false);
 const c=m.confirmation!,recipients=m.mentions.filter(v=>v.kind==='human'&&v.requiresResponse),mine=c.responses.find(r=>r.userId===w.user.id);
 const allowed=recipients.some(v=>v.targetId===w.user.id)&&!mine&&!c.closed;
 const activeRole=m.roleAgentId&&w.group?.members.some(p=>p.roles.some(r=>r.id===m.roleAgentId));
 const label=(value:Choice)=>w.t(value==='agree'?'confirmAgree':value==='changes'?'confirmChanges':'confirmComment');
 const name=(id:string)=>w.group?.members.find(p=>p.userId===id)?.displayName||w.t('confirmFormerMember');
 const respond=async()=>{if(!allowed||w.pending||choice==='comment'&&!feedback.trim())return;try{await w.execute(`/conversations/${w.roomId}/messages`,{replyTo:m.id,body:label(choice)+(feedback.trim()?'\n\n'+feedback.trim():''),decision:{choice,expectedVersion:c.version},mentions:wake&&activeRole?[{kind:'role_agent',targetId:m.roleAgentId}]:[]});setEditing(false);setFeedback('');}catch{/* Preserve feedback until the response has been confirmed. */}};
 return <section className="judex-confirmation" aria-label={w.t('confirmCard')} data-testid="confirmation-card" data-request-id={m.id}>
  <header><ClipboardCheck/><strong>{c.title||w.t('confirmTitle')}</strong><span>{c.closed?w.t('confirmClosed'):c.responses.length===recipients.length?w.t('confirmComplete'):w.t('confirmPending')}</span></header>
  <div className="judex-confirmation-body"><dl><dt>{w.t('confirmFrom')}</dt><dd>{m.authorName}{m.positionName?' · '+m.positionName:''}</dd><dt>{w.t('confirmTo')}</dt><dd>{recipients.map(p=>name(p.targetId)).join('、')}</dd></dl><MarkdownReader text={m.body}/>{m.files.map(f=><FileCard key={f.id} file={f}/>)}</div>
  {c.responses.length>0&&<div className="judex-confirmation-responses">{c.responses.map(r=><details key={r.userId}><summary><Check/><strong>{name(r.userId)}</strong><span>{label(r.choice)}</span><time dateTime={r.createdAt}>{new Date(r.createdAt).toLocaleString(w.locale)}</time></summary><p>{r.body}</p></details>)}</div>}
  {allowed&&<>{editing&&<div className="judex-confirmation-feedback"><Field label={w.t('confirmFeedback')} value={feedback} onChange={setFeedback} multiline rows={3}/>{activeRole&&<CheckField selected={wake} onChange={setWake}>{w.t('confirmWake',{name:m.positionName||m.authorName})}</CheckField>}</div>}
  <footer><div className="judex-confirmation-split"><Button isDisabled={w.pending||choice==='comment'&&!feedback.trim()} onPress={()=>void respond()}><Send/>{wake?w.t('confirmSubmitWake'):label(choice)}</Button><ActionMenu label={w.t('confirmChoose')} items={(['agree','changes','comment'] as Choice[]).map(value=>({id:value,label:label(value),icon:value===choice?<Check/>:<Pencil/>,action:()=>{setChoice(value);setEditing(true);}}))}><ChevronDown/></ActionMenu></div><Button aria-expanded={editing} onPress={()=>setEditing(!editing)}><Pencil/>{w.t('confirmEdit')}</Button><span>{w.t('confirmOpinionOnly')}</span></footer></>}
 </section>;
}
