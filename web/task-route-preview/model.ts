export type Text={zh:string;en:string};
export const tx=(zh:string,en:string):Text=>({zh,en});
export type Stage='draft'|'ready'|'working'|'delivered'|'accepted'|'rework';
export type Section='overview'|'records'|'flow';
export type Plan={id:string;projectId:string;title:Text;goal:Text;criteria:Text[];owner:string;status:'draft'|'active'|'accepted';version:number;pending?:boolean};
export type Condition={id:string;kind:'acceptance'|'material'|'receipt';target:string;label:Text;met?:boolean};
export type Task={id:string;planId:string;title:Text;description:Text;criteria:Text[];owner:string;reviewer:string;status:Stage;before:string[];conditions:Condition[];version:number;pending?:boolean;skip?:{reason:string;at:string;waivers:string[]};workflowAssigned:boolean;flowNode:Text;responsibility:Text};
export type RecordItem={id:string;taskId:string;kind:'progress'|'delivery'|'decision';author:string;source:'web'|'cli';at:string;summary:Text|string;original:Text|string;analysis?:Text;materials?:Text[]};
export type Patch={title:Text;description:Text;criteria:Text[];owner:string;reviewer?:string;before?:string[]};
export type Change={type:'task'|'plan';id:string;patch:Patch;version:number};
export type State={schema:4;role:'manager'|'member';plans:Plan[];tasks:Task[];records:RecordItem[];changes:Change[]};
export type Page={kind:'projects'|'plans'|'route'|'chat';projectId?:string;planId?:string;taskId?:string};
export type ModalState={kind:'edit'|'delete'|'review';type:'task'|'plan';id:string}|{kind:'skip'|'restore'|'report';id:string}|{kind:'create';id:string};
export const STORAGE='judex.demo4.v1';
export const workflowNodes=[tx('需求确认','Confirm requirements'),tx('交互评审','Review interactions'),tx('功能开发','Implement features'),tx('数据契约核对','Check data contracts'),tx('联调与验收','Integration and acceptance')];
export const people={manager:tx('林然','Lin Ran'),member:tx('陈禾','Chen He'),designer:tx('顾言','Gu Yan'),reviewer:tx('杜衡','Du Heng')};
export const projectNames={product:tx('轻笺 · 协同平台','Lightnote · Collaboration'),research:tx('体验研究','Experience research')};
export function seed():State{
 const plans:Plan[]=[
  {id:'launch',projectId:'product',title:tx('首版桌面协同体验','First desktop collaboration release'),goal:tx('让计划、任务与讨论之间的切换清晰自然。','Make moving between plans, tasks, and discussions clear and natural.'),criteria:[tx('核心协作链路可以从创建到验收完整走通','Complete the core collaboration journey from creation to acceptance'),tx('中英文、深浅色下保持一致的桌面体验','Consistent desktop experience in both languages and themes')],owner:'manager',status:'active',version:3},
  {id:'quality',projectId:'product',title:tx('下一轮体验与质量检查','Next experience and quality review'),goal:tx('补齐关键场景，安排独立的体验和性能检查。','Cover key scenarios with focused experience and performance checks.'),criteria:[tx('形成可复现的检查清单和问题记录','Produce a reproducible checklist and issue records')],owner:'manager',status:'draft',version:1,pending:true},
  {id:'research',projectId:'research',title:tx('团队访谈与需求整理','Team interviews and requirements'),goal:tx('整理协作中的真实问题，形成下一阶段需求。','Identify real collaboration problems for the next stage.'),criteria:[tx('完成访谈整理并确认关键需求','Summarize interviews and confirm key needs')],owner:'member',status:'draft',version:1},
 ];
 const base=(id:string,planId:string,title:Text,description:Text,status:Stage,before:string[],owner='member'):Task=>({id,planId,title,description,status,before,owner,reviewer:'manager',version:1,criteria:[tx('成果清晰可核对，相关问题已有明确处理','Output is reviewable and related issues are explicitly addressed')],conditions:[],workflowAssigned:planId==='launch'&&status!=='draft',flowNode:title,responsibility:description});
 const tasks=[
  base('needs','launch',tx('需求确认','Confirm requirements'),tx('整理用户反馈，明确本轮范围、交付内容与验收标准。','Review user feedback and agree on scope, outputs, and acceptance criteria.'),'accepted',[],'manager'),
  base('review','launch',tx('交互评审','Review interactions'),tx('评审任务详情、讨论侧栏和路线查看，收敛关键交互。','Review task details, discussion panels, and route exploration.'),'working',['needs'],'designer'),
  base('build','launch',tx('功能开发','Implement features'),tx('完成任务生命周期与共用详情面板，接入正式业务接口。','Implement task lifecycle and shared detail panels using formal APIs.'),'ready',['review']),
  base('contract','launch',tx('数据契约核对','Check data contracts'),tx('对照页面需要的信息，核对状态、权限和依赖的数据契约。','Check state, permission, and dependency contracts against UI needs.'),'ready',['review'],'manager'),
  base('acceptance','launch',tx('联调与验收','Integration and acceptance'),tx('验证完整协作链路，检查中英文、深浅色和关键异常路径。','Verify the collaboration journey, both themes and languages, and exception paths.'),'ready',['build','contract'],'reviewer'),
  base('draft-check','launch',tx('补充性能检查','Additional performance checks'),tx('检查大计划路线、长记录与多标签切换，明确性能目标。','Check large routes, long histories, and tab switching against performance goals.'),'draft',['build']),
  base('quality-check','quality',tx('体验检查清单','Experience checklist'),tx('整理任务创建、详情查看与讨论切换的检查步骤。','Write checks for task creation, detail viewing, and discussion navigation.'),'draft',[]),
  base('quality-report','quality',tx('问题复盘','Review findings'),tx('按影响整理问题和改进建议，明确下一轮优先级。','Group findings and recommendations by impact and priority.'),'draft',['quality-check']),
 ];
 tasks.find(t=>t.id==='build')!.conditions=[{id:'build-review',kind:'acceptance',target:'review',label:tx('交互评审已验收','Interaction review accepted')}];
 tasks.find(t=>t.id==='acceptance')!.conditions=[{id:'acceptance-build',kind:'acceptance',target:'build',label:tx('功能开发已验收','Implementation accepted')},{id:'acceptance-material',kind:'material',target:'checklist',label:tx('联调检查清单已就绪','Integration checklist ready'),met:true}];
 tasks.find(t=>t.id==='draft-check')!.pending=true;
 return {schema:4,role:'manager',plans,tasks,changes:[],records:[
  {id:'r-review-2',taskId:'review',kind:'progress',author:'designer',source:'cli',at:'2026-10-08T14:20:00+08:00',summary:tx('任务卡片与详情抽屉方案已整理，正在核对跳过后的后继条件。','Task cards and the detail drawer are drafted; successor conditions are under review.'),original:tx('已完成：任务卡片信息层级、详情抽屉布局和讨论右栏功能入口。\n待确认：跳过任务时，需明确区分普通顺序与验收条件。\n下一步：用可交互原型核对修改、删除和恢复执行的确认方式。','Completed: task card hierarchy, detail drawer layout, and discussion panel entries.\nPending: distinguish sequence bypass from acceptance requirements when skipping.\nNext: review editing, deletion, and resuming through an interactive prototype.'),analysis:tx('展示层级已明确。运行例外需要保留依据，并逐项核对受影响的后继条件。','Presentation hierarchy is clear. Execution exceptions need recorded reasons and a review of successor conditions.'),materials:[tx('任务详情布局草图.pdf','Task detail layout.pdf')]},
  {id:'r-review-1',taskId:'review',kind:'progress',author:'designer',source:'web',at:'2026-10-08T10:10:00+08:00',summary:tx('已整理七项交互调整，任务信息与流程参考分别组织。','Seven interaction changes are outlined; task information and workflow reference are separated.'),original:tx('本轮重点是降低操作密度，保留清晰的对象范围和一个主要操作。','Focus on reducing action density while retaining clear object scope and one primary action.')},
  {id:'r-needs',taskId:'needs',kind:'decision',author:'manager',source:'web',at:'2026-10-08T09:00:00+08:00',summary:tx('已确认本轮范围和完成条件。','Scope and completion criteria confirmed.'),original:tx('本轮先核对交互原型；正式实现继续保留职责、审批和验收边界。','Review the interaction prototype first; formal implementation retains responsibility, approval, and acceptance boundaries.')},
 ]};
}
export function canEdit(s:State,t:Task|Plan){return s.role==='manager'||t.owner==='member';}
export function canWork(s:State,t:Task){return t.owner===s.role;}
export function effectivePredecessors(s:State,t:Task,visited=new Set<string>()):string[]{
 if(visited.has(t.id))return [];visited.add(t.id);
 return [...new Set(t.before.flatMap(id=>{const prev=s.tasks.find(v=>v.id===id);return prev?.skip?effectivePredecessors(s,prev,new Set(visited)):[id];}))];
}
export function unmet(s:State,t:Task){
 const blockers=effectivePredecessors(s,t).flatMap(id=>{const v=s.tasks.find(t=>t.id===id);return v&&v.status!=='accepted'?[v.title]:[];});
 for(const c of t.conditions){const prev=s.tasks.find(v=>v.id===c.target);const waived=prev?.skip?.waivers.includes(c.id);if(!waived&&(c.kind==='acceptance'?prev?.status!=='accepted':!c.met)&&!blockers.some(b=>prev&&b.zh===prev.title.zh))blockers.push(c.kind==='acceptance'&&prev?prev.title:c.label);}
 return blockers;
}
export function taskStage(s:State,t:Task){return t.skip?'skipped':t.status==='ready'&&unmet(s,t).length?'blocked':t.status;}
export function stats(s:State,p:Plan){const tasks=s.tasks.filter(t=>t.planId===p.id);return {total:tasks.filter(t=>!t.skip&&t.status!=='draft').length,accepted:tasks.filter(t=>t.status==='accepted'&&!t.skip).length,drafts:tasks.filter(t=>t.status==='draft').length,skipped:tasks.filter(t=>t.skip).length};}
export function descendants(s:State,id:string):Task[]{const seen=new Set<string>();const walk=(target:string)=>{for(const t of s.tasks.filter(t=>t.before.includes(target)||t.conditions.some(c=>c.target===target))){if(seen.has(t.id))continue;seen.add(t.id);walk(t.id);}};walk(id);return s.tasks.filter(t=>seen.has(t.id));}
export function deleteBlocked(s:State,type:'task'|'plan',id:string){const targets=type==='task'?[id]:s.tasks.filter(t=>t.planId===id).map(t=>t.id);return (type==='plan'&&s.plans.find(p=>p.id===id)?.status!=='draft')||targets.some(t=>s.tasks.find(v=>v.id===t)?.status!=='draft')||s.tasks.some(t=>!targets.includes(t.id)&&(t.before.some(b=>targets.includes(b))||t.conditions.some(c=>targets.includes(c.target))));}
export function hasCycle(s:State,id:string,before:string[]){return before.some(b=>b===id||descendants(s,id).some(t=>t.id===b));}
