import { Button } from "../../components/ui/Button";
import { useWork } from "../work/store";
import {EmptyState} from "../../components/ui/Presentation";
export function ProjectConversationOverview(){
 const {state,project,t,text,go}=useWork();
 return <section className="judex-collab-overview">
  <div className="judex-co-conversation-title"><div><h2>{t("coopHistory")}</h2><p>{text(project.description)}</p></div><Button size="sm" onPress={()=>go({editor:"topic"})}>{t("coopNewDiscussion")}</Button></div>
  <div className="judex-collab-overview-body">
   <EmptyState title={t("coopHistoryHint")}/>
  </div>
 </section>;
}
