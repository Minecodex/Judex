import {Dropdown,Label} from "@heroui/react";
import {Link,MoreHorizontal,GitFork,Sparkles,GitBranch} from "lucide-react";
import {Button} from "../../components/ui/Button";
import {useWork} from "../work/store";
import type {Topic} from "../work/types";
import {effectivePlanIds} from "../cooperation/scope";
export function ConversationHeading({topic,limited,onFork,onArrange}:{topic:Topic;limited:boolean;onFork:()=>void;onArrange:()=>void}){
 const {state,t,text,go,act}=useWork();
 const planMain=state.plans.some(v=>v.mainTopicId===topic.id),taskMain=state.tasks.some(v=>v.mainTopicId===topic.id),plans=effectivePlanIds(topic,state.tasks);
 return <><div className="judex-co-conversation-title"><div><h2>{text(topic.title)}</h2><span className="judex-co-meta">{t(planMain||taskMain?"coMainDiscussion":"coSpecialDiscussion")}</span></div><div className="judex-co-actions"><Button size="sm" variant="ghost" data-testid="chat-discuss" disabled={limited} aria-label={t("chatAnalyze")} title={t("chatAnalyze")} onPress={()=>void act("discussTopic",{topicId:topic.id},{toast:false})}><Sparkles/></Button><Button size="sm" variant="ghost" aria-label={t("coopLinkObjects")} title={t("coopLinkObjects")} onPress={()=>go({editor:"topic",editTopicId:topic.id})}><Link/></Button><Dropdown><Button size="sm" variant="ghost" data-testid="conversation-menu" aria-label={t("coopDiscussionMenu")}><MoreHorizontal/></Button><Dropdown.Popover><Dropdown.Menu onAction={key=>key==="fork"?onFork():key==="settings"?go({settingsSection:"project"}):onArrange()}><Dropdown.Item id="fork" textValue={t("coFork")}><GitFork/><Label>{t("coFork")}</Label></Dropdown.Item><Dropdown.Item id="arrange" data-testid="chat-propose" textValue={t("chatPropose")}><GitBranch/><Label>{t("chatPropose")}</Label></Dropdown.Item><Dropdown.Item id="settings" data-testid="conversation-project-settings" textValue={t("coopProjectSettings")}><Label>{t("coopProjectSettings")}</Label></Dropdown.Item></Dropdown.Menu></Dropdown.Popover></Dropdown></div></div>
  <div className="judex-co-topic-links">{plans.map(id=>{const p=state.plans.find(v=>v.id===id);return p&&<Button key={id} size="sm" variant="ghost" onPress={()=>go({page:"route",scopePlanId:id,scopeTaskId:undefined,view:"plan",id})}>{text(p.title)}</Button>;})}{topic.taskIds.map(id=>{const task=state.tasks.find(v=>v.id===id);return task&&<Button key={id} size="sm" variant="ghost" onPress={()=>go({view:"task",id})}>{text(task.title)}</Button>;})}{plans.length>1&&<span className="judex-co-meta">{t("coopSameTopic")}</span>}</div>
 </>;
}
export function MessageMenu({seq,onFork,onSource}:{seq?:number;onFork:()=>void;onSource?:()=>void}){
 const {t}=useWork();return <Dropdown><Button size="sm" variant="ghost" aria-label={t("coForkHere")+" · "+seq}><MoreHorizontal size={14}/></Button><Dropdown.Popover><Dropdown.Menu onAction={key=>key==="source"?onSource?.():onFork()}><Dropdown.Item id="fork" textValue={t("coForkHere")}><Label>{t("coForkHere")}</Label></Dropdown.Item>{onSource&&<Dropdown.Item id="source" textValue={t("coGoSource")}><Label>{t("coGoSource")}</Label></Dropdown.Item>}</Dropdown.Menu></Dropdown.Popover></Dropdown>;
}
