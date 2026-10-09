import {createContext, useContext, useState, type ReactNode} from 'react';
import {useWork} from '../work/store';
import {materialEvidence, type MaterialItem} from './api';
import {conversationContextKey, storedConversationContext, type ConversationDraft, type ConversationWorkContext} from '../chat/conversationDraft';
import {ShareMaterialDialog} from './ShareMaterialDialog';
type ShareTarget = {topicId: string; taskId?: string; planId?: string};
type Sharing = {open: (file: MaterialItem) => void; staged?: {key: string; revision: number}; context: (topicId:string)=>ConversationWorkContext|undefined; rememberContext: (topicId:string,context:ConversationWorkContext)=>void};
const SharingContext = createContext<Sharing | null>(null);
export function useMaterialSharing() {return useContext(SharingContext);}
export function MaterialSharingProvider({draftCache, children}: {draftCache: Map<string, ConversationDraft>; children: ReactNode}) {
  const {project, state, go} = useWork(), [file, setFile] = useState<MaterialItem>(), [staged, setStaged] = useState<Sharing['staged']>();
  const [,refreshContexts]=useState(0),contextKey=(topicId:string)=>project.id+':'+(state.currentUserId??state.currentUser)+':'+topicId;
  const rememberContext=(topicId:string,context:ConversationWorkContext)=>{
    const key=contextKey(topicId),old=draftCache.get(key);
    if(old)draftCache.set(key,{...old,context});
    try{sessionStorage.setItem(conversationContextKey(key),JSON.stringify(context));}catch{}
    refreshContexts(revision=>revision+1);
  };
  const stage = (target: ShareTarget) => {
    if (!file || file.deletedAt) return;
    const actor = state.currentUserId ?? state.currentUser, key = project.id + ':' + actor + ':' + target.topicId;
    const old = draftCache.get(key) ?? {body: sessionStorage.getItem('judex.chat.draft.' + key) ?? '', files: [], attachments: false};
    const evidence = materialEvidence(file), files = old.files.some(v => v.versionId === evidence.versionId) ? old.files : [...old.files, evidence];
    const context:ConversationWorkContext=target.taskId?{kind:'task',id:target.taskId}:target.planId?{kind:'plan',id:target.planId}:{kind:'topic'};
    draftCache.set(key, {...old, files, attachments: true, context});
    try{sessionStorage.setItem(conversationContextKey(key),JSON.stringify(context));}catch{}
    setStaged(previous => ({key, revision: (previous?.revision ?? 0) + 1}));setFile(undefined);
    go({page: 'chat', view: 'topic', id: target.topicId, conversation: target.topicId,
      scopeTaskId: target.taskId, scopePlanId: target.taskId ? undefined : target.planId,
      taskContextId: target.taskId, settingsSection: undefined});
  };
  return <SharingContext.Provider value={{open: setFile, staged,rememberContext,context:topicId=>draftCache.get(contextKey(topicId))?.context??storedConversationContext(contextKey(topicId))}}>{children}{file && <ShareMaterialDialog file={file} onClose={() => setFile(undefined)} onStage={stage}/>}</SharingContext.Provider>;
}
