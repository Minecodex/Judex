import { useInfiniteQuery } from "@tanstack/react-query";
import { request } from "../../lib/api/client";
import type { components } from "../../lib/api/schema";
import { useWork } from "../work/store";
import { mapTopic } from "../work/apiModel";
import type { Topic } from "../work/types";
import { collaborationKey } from "./collaborationData";
import { demoHistory } from "./demoCollaboration";
type Message=components["schemas"]["Message"];
type Page={items:Message[];nextCursor:string|null};
export function useConversationHistory(topic:Topic){
 const {state,project,mode}=useWork();
 const query=useInfiniteQuery({
  queryKey:collaborationKey(project.id,state.currentUserId??state.currentUser,"history",topic.id),enabled:mode==="api",
  initialPageParam:"",queryFn:({pageParam})=>request<Page>(`/projects/${project.id}/topics/${topic.id}/messages?limit=50${pageParam?"&cursor="+encodeURIComponent(pageParam):""}`),
  getNextPageParam:page=>page.nextCursor||undefined,refetchInterval:5000,
 });
 const raw=query.data?.pages.slice().reverse().flatMap(p=>p.items)??[];
 const unique=[...new Map(raw.map(m=>[m.id,m])).values()].sort((a,b)=>a.seq-b.seq);
 const messages=mode==="demo"?demoHistory(state,topic):mapTopic({id:topic.id,title:"",kind:topic.kind??"discussion",createdAt:""},project.id,unique).messages;
 return {...query,messages,pending:mode==="api"&&query.isPending};
}
