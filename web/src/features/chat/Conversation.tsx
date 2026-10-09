import {MaterialReferences} from "../materials/FileCard";
import {MaterialPicker} from "../materials/MaterialLibrary";
import {useMaterialSharing} from '../materials/MaterialSharing';
import {useConversationHistory} from "./conversationData";
import {ForkDialog} from "./CollaborationDialogs";
import {PlanConversationCards,TaskConversationCards} from "./CollaborationCards";
import {UIOption,UISelect,UIWarning} from "../../components/ui/FormControls";
import {GitFork} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  ArrowUp,
  ArrowUpRight,
  Paperclip,
  Sparkles,
  GitBranch,
} from "lucide-react";
import { TextArea } from "@heroui/react";
import { PersonAvatar } from "../../components/ui/Presentation";
import { useWork } from "../work/store";
import { Button } from "../../components/ui/Button";
import { Person, Upload, EvidenceList } from "../work/ui";
import type { Topic, Evidence } from "../work/types";
import { latestDiscussionRun } from "./discussionPolicy";
import {ProposalSummary} from "./ProposalSummary";
import { useReadingPosition } from "./useReadingPosition";
import {ConversationHeading,MessageMenu} from "./ConversationHeading";
import {discussionTasks} from "../cooperation/scope";
import {initialConversationContext, conversationContextKey, conversationContextValue, conversationContextFromValue, type ConversationDraft, type ConversationWorkContext} from './conversationDraft';
export type {ConversationDraft} from './conversationDraft';
export function Conversation({
  topic,
  cache,
}: {
  topic: Topic;
  cache: Map<string, ConversationDraft>;
}) {
  const { state, project, route, t, text, act, go, mode } = useWork();
 const sharing=useMaterialSharing();
 const history=useConversationHistory(topic);
 const messages=history.messages;
 const plan=state.plans.find(p=>p.mainTopicId===topic.id);
 const taskMain=state.tasks.find(v=>v.mainTopicId===topic.id);
 const allowedTasks=discussionTasks(topic,state.tasks);
 const allowedPlans=state.plans.filter(v=>topic.planIds.includes(v.id));
 const parent=state.topics.find(p=>p.id===topic.parentTopicId);
 const [choosingFiles,setChoosingFiles]=useState(false);
 const [fork,setFork]=useState<{seq?:number}|null>(null);
 const actor=state.currentUserId??state.currentUser;
  const key =
    project.id + ":" + actor + ":" + (topic?.id ?? "home");
 const [context,setContext]=useState<ConversationWorkContext>(()=>initialConversationContext(key,cache,route));
 const taskContext=context.kind==='task'?context.id:'';
 useEffect(()=>{if(route.taskContextId!==(taskContext||undefined))go({taskContextId:taskContext||undefined});},[taskContext,route.taskContextId]);
  const [body, setBody] = useState(
      () =>
        cache.get(key)?.body ??
        sessionStorage.getItem("judex.chat.draft." + key) ??
        "",
    ),
    [files, setFiles] = useState<Evidence[]>(() => cache.get(key)?.files ?? []),
    [attachments, setAttachments] = useState(
      () => cache.get(key)?.attachments ?? false,
    );
  useEffect(()=>{if(sharing?.staged?.key!==key)return;const draft=cache.get(key);if(draft){setFiles(draft.files);setAttachments(draft.attachments);setContext(draft.context??{kind:'topic'});}},[sharing?.staged?.revision,key]);
  const tail = useRef<HTMLDivElement>(null);
  const sending=useRef(false),[sendBusy,setSendBusy]=useState(false);
  const reading = useReadingPosition(key, !history.pending);
  const previousSeq = useRef<number|undefined>(undefined);
  const followTail = useRef(true);
  const jumped = useRef<string|undefined>(undefined);
  useEffect(() => {
    cache.set(key, { body, files, attachments,context });
  }, [key, body, files, attachments, cache,context]);
  useEffect(() => {
    sessionStorage.setItem("judex.chat.draft." + key, body);
  }, [key, body]);
  useEffect(()=>{if(sharing)sharing.rememberContext(topic.id,context);else try{sessionStorage.setItem(conversationContextKey(key),JSON.stringify(context));}catch{}},[key,context]);
  useEffect(() => {
    const el=reading.current;
    if(!el)return;
    const track=()=>{followTail.current=el.scrollHeight-el.scrollTop-el.clientHeight<el.clientHeight/4;};
    track();el.addEventListener("scroll",track);
    return ()=>el.removeEventListener("scroll",track);
  },[]);
  useEffect(() => {
    const latest=messages.at(-1)?.seq;
    if(previousSeq.current!==undefined && latest!==undefined && latest>previousSeq.current && followTail.current && route.messageSeq===undefined)
      tail.current?.scrollIntoView({block:"nearest"});
    if(latest!==undefined)previousSeq.current=latest;
  },[messages.at(-1)?.seq]);
  useEffect(()=>{
    const seq=route.messageSeq;
    const jumpKey=topic.id+":"+seq;
    if(seq===undefined||history.pending||jumped.current===jumpKey)return;
    const el=reading.current?.querySelector<HTMLElement>('[data-message-seq="'+seq+'"]');
    if(el){el.scrollIntoView({block:"center"});jumped.current=jumpKey;}
    else if(history.hasNextPage&&!history.isFetchingNextPage)void history.fetchNextPage();
  },[route.messageSeq,history.pending,messages.length,history.hasNextPage,history.isFetchingNextPage,topic.id]);
  const send = async () => {
    if(!validContext||sending.current)return;
    sending.current=true;setSendBusy(true);
    try{
    const r = await act(
      "sendTopicMessage",
      {
        projectId: project.id,
        topicId: topic?.id,
        taskId:taskContext||undefined,
        planId:context.kind==='plan'?context.id:undefined,
        title: body.trim().slice(0, 48) || t("chatHome"),
        body,
        files,
      },
      { toast: false },
    );
    if (r.ok) {
      if (r.id && r.id !== topic?.id) go({ view: "topic", id: r.id });
      sessionStorage.removeItem("judex.chat.draft." + key);
      setBody("");
      setFiles([]);
      setAttachments(false);
      cache.set(key,{body:"",files:[],attachments:false,context});
    }
    }finally{sending.current=false;setSendBusy(false);}
  };
  const run = topic ? latestDiscussionRun(state, topic.id) : undefined;
  const atLimit = !!run && run.rounds >= run.maxRounds;
  const validContext=context.kind==='topic'||context.kind==='task'&&allowedTasks.some(v=>v.id===context.id)||context.kind==='plan'&&allowedPlans.some(v=>v.id===context.id);
  const proposals = (state.proposals ?? []).filter(
    (p) => p.topicId === topic?.id,
  );
  return (
    <section className="judex-chat-conversation judex-collab-conversation judex-co-conversation">
      <div className="judex-collab-conversation-heading">
        <ConversationHeading topic={topic} limited={atLimit} onFork={()=>setFork({seq:topic.lastMessageSeq??messages.at(-1)?.seq??0})} onArrange={()=>go({editor:'proposal',proposalTopicId:topic.id})}/>
        {topic.parentTopicId&&<div className="judex-collab-lineage">{t("coForkSource")}：<Button size="sm" variant="ghost" onPress={()=>go({view:"topic",id:topic.parentTopicId,scopePlanId:undefined,scopeTaskId:undefined,taskContextId:undefined,messageSeq:topic.forkAfterSeq})}>{parent?text(parent.title):t("coParentMissing")} · {t("coForkPoint",{seq:topic.forkAfterSeq??0})}</Button></div>}
        {topic.sourceRefs?.filter(ref=>ref.type!=="message").map(ref=><Button key={ref.type+ref.id} size="sm" variant="ghost" className="judex-collab-source-link" onPress={()=>go({view:"task",id:ref.taskId??topic.taskIds[0],activityId:ref.id})}>{t("coOriginalRecord")} · {text(state.tasks.find(v=>v.id===(ref.taskId??topic.taskIds[0]))?.title??"")}</Button>)}
      </div>
      <div
        className="judex-chat-thread judex-co-thread"
        data-testid="chat-thread"
        ref={reading}
      >
        {history.pending?<p role="status">{t("shellLoading")}</p>:history.isError?<UIWarning>{t("errNetwork")}<Button onPress={()=>void history.refetch()}>{t("shellRetry")}</Button></UIWarning>:null}
 {history.hasNextPage&&<Button size="sm" variant="secondary" onPress={()=>void history.fetchNextPage()}>{t("coMoreRecords")}</Button>}
 {!messages.length&&!plan&&!history.pending ? <div className="judex-chat-intro">
          <span className="judex-chat-orbit"><Sparkles /></span>
          <h2>{text(topic.title)}</h2>
          <p>{t("coopAssociationHint")}</p>
          <small>{t("portalAIHint")}</small>
        </div> : <div className="judex-chat-date"><span />{t("today")}<span /></div>}
        {messages.map((m,index) => (
 <div key={m.id}>
 {topic.parentTopicId&&!m.inherited&&(index===0||messages[index-1].inherited)&&<div className="judex-collab-branch-start">{t("coBranchStart")}</div>}
          <article
            className={"judex-chat-message judex-co-message judex-chat-message-" + m.kind+(m.inherited?" judex-collab-inherited":"")+(route.messageSeq===m.seq?" judex-collab-message-focus":"")}
 data-message-seq={m.seq}
            key={m.id}
          >
            {m.kind === "ai" ? <span className="judex-ai-avatar"><Sparkles /></span> : <PersonAvatar name={m.actor} />}
            <div className="judex-chat-message-content judex-co-message-content">
            <div className="judex-chat-message-by judex-co-message-by">
              <strong>{m.actor}</strong>
              {m.seatId && (
                <span>
                  {text(
                    state.positions.find(
                      (p) =>
                        p.id ===
                        state.seats.find((s) => s.id === m.seatId)?.positionId,
                    )?.name ?? "",
                  )}
                </span>
              )}
              <span>
                {m.kind === "ai"
                  ? t("workAI")
                  : new Date(m.at).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
              </span>
            </div>
            <div className="judex-chat-message-body">{text(m.text)}</div>
            <div className="judex-collab-message-actions">
              {m.taskId&&<span className="judex-collab-tag">{text(state.tasks.find(v=>v.id===m.taskId)?.title??"")}</span>}
              {m.inherited&&<span className="judex-collab-meta">{t("coInherited")}</span>}
              <MessageMenu seq={m.seq} onFork={()=>setFork({seq:m.seq})} onSource={m.inherited&&m.originTopicId?()=>go({view:"topic",id:m.originTopicId,scopePlanId:undefined,scopeTaskId:undefined,taskContextId:undefined,messageSeq:m.seq}):undefined}/>

            </div>
            {!!m.files?.length && <EvidenceList files={m.files} />}
            {mode==="api"&&!!m.materials?.length&&<MaterialReferences materials={m.materials}/>}
            </div>
          </article>
 </div>
        ))}
 {plan&&<PlanConversationCards planId={plan.id}/>}
 {taskMain&&<TaskConversationCards taskId={taskMain.id}/>}
        {proposals.map((p) => (
          <ProposalSummary key={p.id} proposal={p} />
        ))}
        <div ref={tail} />
      </div>
      <div className="judex-chat-compose judex-co-composer">
        {run && (
          <div
            className="judex-discussion-budget"
            role="status"
            data-testid="discussion-budget"
          >
            <strong>
              {t("chatRoundCount", { used: run.rounds, max: run.maxRounds })}
            </strong>
            <span>
              {t(atLimit ? "chatRoundLimitReached" : "chatRoundWaiting")}
            </span>
          </div>
        )}

        <div className="judex-chat-input-wrap">
          {(allowedTasks.length>0||allowedPlans.length>0||context.kind!=='topic')&&<div className="judex-collab-compose-scope"><label>{t("coScope")}</label><UISelect data-testid="collaboration-compose-task" value={conversationContextValue(context)} onChange={e=>setContext(conversationContextFromValue(e.target.value))} aria-label={t("coScope")}><UIOption value="">{t("coopWholeConversation")}</UIOption>{allowedPlans.map(v=><UIOption key={'plan:'+v.id} value={'plan:'+v.id}>{t('matPlan')} · {text(v.title)}</UIOption>)}{allowedTasks.map(v=><UIOption key={v.id} value={v.id}>{text(v.title)}</UIOption>)}</UISelect></div>}
          {!validContext&&<UIWarning>{t("coopContextChanged")}</UIWarning>}
          <TextArea
            data-testid="work-discussion-input"
            aria-label={t("chatMessage")}
            value={body}
            disabled={sendBusy}
            placeholder={t("chatMessage")}
            onChange={(e) => setBody(e.target.value)}
            onKeyDown={(e) => {
              if (
                e.key === "Enter" &&
                !e.shiftKey && !e.nativeEvent.isComposing &&
                (body.trim() || files.length)
              ) {
                e.preventDefault();
                void send();
              }
            }}
          />
          <div>
            <Button
              aria-label={t("chatAttach")}
              onClick={() => setAttachments(!attachments)}
            >
              <Paperclip size={18} />
            </Button>
            <Button
              variant="primary"
              className="judex-chat-send"
              data-testid="send-work-message"
              isPending={sendBusy}
              aria-label={t("chatSend")}
              disabled={!validContext||!body.trim() && !files.length}
              onClick={send}
            >
              {t("chatSend")}<ArrowUp size={19} />
            </Button>
          </div>
        </div>
  {choosingFiles&&<MaterialPicker onClose={()=>setChoosingFiles(false)} onSelect={selected=>{setFiles(previous=>[...previous,...selected.filter(f=>!previous.some(v=>v.id===f.id))]);setAttachments(true);}}/>}
      {attachments && <><Upload files={files} onChange={setFiles} />{mode==="api"&&<Button size="sm" variant="outline" onPress={()=>setChoosingFiles(true)}>{t("matChoose")}</Button>}</>}
        <small>{t("portalComposeHint")} · {t("chatDraftSaved")}</small>
      </div>
      {fork&&<ForkDialog topic={topic} afterSeq={fork.seq} onClose={()=>setFork(null)}/>}
    </section>
  );
}
