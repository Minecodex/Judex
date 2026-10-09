import {W,type State} from "./model";
const rules=[W("成果与约定范围一致","Delivery matches the agreed scope"),W("证据齐全，可复核","Evidence is complete and reviewable")];
export function seed():State{
 return {
  schema:1,userId:"lin",
  people:[{id:"lin",name:W("林然","Lin Ran"),positionId:"owner"},{id:"gu",name:W("顾言","Gu Yan"),positionId:"dev"},{id:"du",name:W("杜衡","Du Heng"),positionId:"qa"},{id:"song",name:W("宋雅","Song Ya"),positionId:"content"}],
  projects:[
   {id:"launch",title:W("轻笺","Lightnote"),goal:W("把轻笺的第一版交到大家手里。","Bring the first version of Lightnote to everyone."),ownerId:"lin",memberIds:["lin","gu","du","song"],status:"active"},
   {id:"cafe",title:W("野间咖啡","Field Café"),goal:W("让新的门店体验自然地落地。","Make a natural café experience a reality."),ownerId:"song",memberIds:["lin","song"],status:"active"},
  ],
  plans:[
   {id:"first",projectId:"launch",title:W("首版上线","First release"),goal:W("完成登录功能、测试核对和使用说明，交付可用的首版。","Deliver a usable release with login, testing, and a user guide."),criteria:rules,ownerId:"lin",status:"active",workflowId:"product",mainTopicId:"main-first"},
   {id:"pilot",projectId:"launch",title:W("用户试点","User pilot"),goal:W("让首批用户顺畅使用，收集实际反馈。","Help early users get started and collect real feedback."),criteria:rules,ownerId:"lin",status:"active",workflowId:"product",mainTopicId:"main-pilot"},
   {id:"foundation",projectId:"launch",title:W("协作准备","Collaboration setup"),goal:W("准备流程、职责和共享资料。","Set up workflows, responsibilities, and shared evidence."),criteria:rules,ownerId:"lin",status:"active",workflowId:"product",mainTopicId:"main-foundation"},
   {id:"observe",projectId:"launch",title:W("上线观测","Release observability"),goal:W("明确异常日志、反馈收集和问题回流的范围。","Define error logs, feedback collection, and issue follow-up."),criteria:rules,ownerId:"lin",status:"draft",workflowId:"product",mainTopicId:"main-observe"},
   {id:"menu",projectId:"cafe",title:W("菜单与体验","Menu & experience"),goal:W("核对门店菜单和首次到店体验。","Review the menu and the first visit."),criteria:rules,ownerId:"song",status:"active",workflowId:"service",mainTopicId:"main-menu"},
  ],
  tasks:[
   {id:"login",projectId:"launch",planId:"first",title:W("登录功能","Login"),goal:W("完成登录、异常提示和可复核的测试记录。","Complete login, error handling, and reviewable test records."),criteria:rules,makerId:"gu",reviewerId:"lin",positionId:"dev",nodeId:"make",status:"delivered",version:2,dependsOn:[]},
   {id:"guide",projectId:"launch",planId:"first",title:W("使用说明","User guide"),goal:W("整理首次使用、成员邀请和常见问题。","Document onboarding, invitations, and common questions."),criteria:rules,makerId:"song",reviewerId:"lin",positionId:"content",nodeId:"make",status:"working",version:1,dependsOn:[]},
   {id:"regression",projectId:"launch",planId:"first",title:W("回归核对","Regression review"),goal:W("接收固定实现版本，核对主要异常场景。","Receive a fixed build and review key error scenarios."),criteria:rules,makerId:"du",reviewerId:"lin",positionId:"qa",nodeId:"review",status:"blocked",version:1,dependsOn:["login"],receiptId:"handoff-login"},
   {id:"release",projectId:"launch",planId:"first",title:W("首版发布核对","Release readiness"),goal:W("核对测试和说明材料，确认发布准备。","Review tests and documentation for release readiness."),criteria:rules,makerId:"gu",reviewerId:"lin",positionId:"dev",nodeId:"deliver",status:"blocked",version:1,dependsOn:["regression","guide"]},
   {id:"interview",projectId:"launch",planId:"pilot",title:W("用户访谈","User interviews"),goal:W("记录试点用户的操作与真实问题。","Record pilot users' actions and real issues."),criteria:rules,makerId:"song",reviewerId:"lin",positionId:"content",nodeId:"make",status:"ready",version:1,dependsOn:[]},
   {id:"setup",projectId:"launch",planId:"foundation",title:W("流程与职责准备","Workflow & responsibilities"),goal:W("规则和职位已核对。","Rules and positions reviewed."),criteria:rules,makerId:"lin",reviewerId:"lin",positionId:"owner",nodeId:"review",status:"accepted",version:2,dependsOn:[]},
   {id:"logs",projectId:"launch",planId:"observe",title:W("异常记录约定","Error logging agreement"),goal:W("明确首版需要保留哪些异常依据。","Specify which error evidence the release needs."),criteria:rules,makerId:"gu",reviewerId:"lin",positionId:"dev",nodeId:"make",status:"draft",version:1,dependsOn:[]},
   {id:"taste",projectId:"cafe",planId:"menu",title:W("门店体验核对","Café experience review"),goal:W("收集试访体验并核对菜单。","Review trial visits and the menu."),criteria:rules,makerId:"song",reviewerId:"song",positionId:"content",nodeId:"visit",status:"working",version:1,dependsOn:[]},
  ],
  topics:[
   {id:"main-first",projectId:"launch",title:W("首版上线 · 主讨论","First release · main"),planIds:["first"],taskIds:[],kind:"plan-main",recordIds:[],messages:[{id:"m1",seq:1,authorId:"lin",kind:"person",at:"09:20",text:W("首版先完成登录功能与使用说明，范围和验收条件按任务卡核对。","Start with login and the guide; review scope and criteria on task cards.")},{id:"m2",seq:2,authorId:"ai",kind:"ai",at:"09:22",text:W("任务记录已经分别汇总。跨计划的衔接可以关联同一场专项讨论，正式安排仍需要相关职责确认。","Task records are summarized separately. Link a focused discussion for cross-plan coordination; formal arrangements still need approval.")}]},
   {id:"main-pilot",projectId:"launch",title:W("用户试点 · 主讨论","User pilot · main"),planIds:["pilot"],taskIds:[],kind:"plan-main",recordIds:[],messages:[{id:"p1",seq:1,authorId:"lin",kind:"person",at:"10:00",text:W("先核对访谈目标，后续用户反馈在对应任务中登记。","Review the interview goal; record later feedback against its task.")}]},
   {id:"main-foundation",projectId:"launch",title:W("协作准备 · 主讨论","Setup · main"),planIds:["foundation"],taskIds:[],kind:"plan-main",recordIds:[],messages:[]},
   {id:"main-observe",projectId:"launch",title:W("上线观测 · 主讨论","Observability · main"),planIds:["observe"],taskIds:[],kind:"plan-main",recordIds:[],parentId:"main-first",forkAfterSeq:1,messages:[]},
   {id:"shared-feedback",projectId:"launch",title:W("首版与试点的反馈衔接","Release and pilot feedback"),planIds:["first","pilot"],taskIds:["login","interview"],kind:"special",recordIds:[],messages:[{id:"share1",seq:1,authorId:"song",kind:"person",at:"10:30",text:W("访谈记录需要能对应首版的异常场景，这场讨论连接两个计划。","Interview notes should connect to release errors. This conversation links both plans.")}]},
   {id:"main-menu",projectId:"cafe",title:W("菜单与体验 · 主讨论","Menu · main"),planIds:["menu"],taskIds:[],kind:"plan-main",recordIds:[],messages:[]},
  ],
  activities:[{id:"report-login",taskId:"login",kind:"delivery",actorId:"gu",source:"cli",text:W("登录流程与异常提示已完成。附上实现与测试记录，请按约定核对。","Login and error messages are ready. Please review the implementation and tests."),materialIds:["login-v2"],at:"11:20",analysis:W("固定交付已登记。尚需验收人核对；发送给测试的版本正在等待接收。","Fixed delivery registered. The reviewer must still accept it; the testing handoff awaits receipt."),topicIds:[]}],
  materials:[{id:"login-v2",projectId:"launch",name:"login-evidence.md",version:2,authorId:"gu",body:W("实现版本：login/review-2。核对项：登录成功、凭据错误、网络重试、重复回调。","Build: login/review-2. Checks: success, invalid credentials, retries, duplicate callbacks.")}],
  suggestions:[],
  decisions:[
   {id:"accept-login",projectId:"launch",kind:"task",title:W("登录功能","Login"),description:W("核对顾言提交的固定交付与异常测试记录。","Review Gu Yan's frozen delivery and error tests."),planId:"first",taskId:"login",sourceTopicId:"main-first",version:1,status:"pending",actors:["lin"],approvedBy:[]},
   {id:"accept-foundation",projectId:"launch",kind:"plan",title:W("协作准备","Collaboration setup"),description:W("必需任务已验收，确认是否满足计划整体目标。","Required tasks are accepted. Confirm the whole plan goal."),planId:"foundation",sourceTopicId:"main-foundation",version:1,status:"pending",actors:["lin"],approvedBy:[]},
   {id:"arrange-observe",projectId:"launch",kind:"arrangement",title:W("上线观测工作安排","Observability arrangement"),description:W("补充异常记录约定，由负责人和实现职责共同确认。","Add an error logging agreement with owner and implementation approval."),planId:"observe",sourceTopicId:"main-first",version:1,status:"pending",actors:["lin","gu"],approvedBy:[]},
  ],
  handoffs:[{id:"handoff-login",projectId:"launch",sourceTaskId:"login",targetTaskId:"regression",senderId:"gu",receiverId:"du",materialIds:["login-v2"],version:2,status:"pending"}],
  positions:[
   {id:"owner",projectId:"launch",name:W("计划负责人","Plan owner"),prompt:W("核对阶段目标、安排和最终成果，保留异议。","Review goals, arrangements, and final evidence; retain objections."),nodes:["review","deliver"]},
   {id:"dev",projectId:"launch",name:W("开发","Development"),prompt:W("实现功能，保留验证依据，明确未完成项。","Implement, retain verification evidence, and state gaps."),nodes:["make","deliver"]},
   {id:"qa",projectId:"launch",name:W("测试核对","Testing"),prompt:W("按固定版本核对异常与完成条件，不把接收当验收。","Review errors and criteria against a fixed version; receipt is not acceptance."),nodes:["review"]},
   {id:"content",projectId:"launch",name:W("内容策划","Content"),prompt:W("整理说明与用户问题，核对可理解性。","Prepare guides and user feedback; check clarity."),nodes:["make"]},
  ],
  flows:[
   {id:"product",projectId:"launch",name:W("首版交付流程","Release delivery workflow"),version:3,nodes:[{id:"make",name:W("实现与制作","Implement & prepare"),responsibility:W("完成具体工作，登记原始成果与验证依据。","Complete work and record original evidence."),positions:["dev","content"]},{id:"review",name:W("接收与核对","Receive & review"),responsibility:W("接收固定版本，核对实际完成条件；验收另外确认。","Receive a fixed version and review criteria; acceptance is confirmed separately."),positions:["qa","owner"]},{id:"deliver",name:W("整体验收","Overall acceptance"),responsibility:W("负责人确认任务与阶段目标，保留正式决定。","The owner checks tasks and goals and records the decision."),positions:["owner"]}],edges:[["make","review"],["review","deliver"]]},
   {id:"service",projectId:"cafe",name:W("体验核对","Experience review"),version:1,nodes:[{id:"visit",name:W("试访与反馈","Trial & feedback"),responsibility:W("核对体验与证据。","Review experience and evidence."),positions:["content"]}],edges:[]},
  ],
  preferences:{},
 };
}
