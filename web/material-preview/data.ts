export type Localized = {zh:string;en:string};
export const l = (zh:string,en:string):Localized=>({zh,en});
export type Format = "pdf"|"docx"|"xlsx"|"png"|"md"|"pptx"|"json"|"zip"|"other";
export type Usage = {scope:"plan"|"task";actor:Localized;time:string;note:Localized};
export type Material = {
 id:string;name:Localized;format:Format;size:string;uploader:Localized;time:string;purpose:Localized;source:Localized;
 version:number;task:boolean;usages:Usage[];deleted?:boolean;local?:boolean;url?:string;textContent?:string;mime?:string;
};
export type Message = {id:string;actor:Localized;time:string;text:Localized;fileIds:string[];scope:"plan"|"task";ai?:boolean};
const lin=l("林晓","Lin Xiao"),zhou=l("周屿","Zhou Yu"),chen=l("陈沐","Chen Mu");
const web=l("网页 · 共享资料","Web · Shared files"),cli=l("本地 CLI · 任务上报","Local CLI · Task report"),chat=l("网页 · 讨论附件","Web · Discussion attachment");
export function seedMaterials():Material[]{
 const base={version:1,task:true,usages:[{scope:"task" as const,actor:lin,time:"2026-10-08 09:45",note:l("作为共享资料卡片和内容预览的设计依据。","Design reference for file cards and content previews.")}]};
 const materials:Material[] = [
  {...base,id:"brief",format:"pdf",name:l("共享资料体验方案.pdf","Shared file experience.pdf"),size:"2.4 MB",uploader:lin,time:"2026-10-08 09:32",purpose:l("说明共享资料、讨论侧栏与消息附件的统一体验，用于本轮设计评审和开发范围确认。","Define a shared experience for the file library, discussion panel and message attachments, for design review and implementation scope."),source:web,
   usages:[...base.usages,{scope:"plan",actor:zhou,time:"2026-10-08 10:05",note:l("在计划主讨论中同步评审材料，供项目成员共同确认。","Shared in the main plan discussion for team review.")}]},
  {...base,id:"requirements",format:"docx",name:l("资料预览需求说明.docx","File preview requirements.docx"),size:"386 KB",uploader:zhou,time:"2026-10-08 09:48",purpose:l("明确文件预览、用途说明和删除引用的交互规则，供前后端实现与验收使用。","Specify preview, purpose and deleted-reference interactions for implementation and acceptance."),source:web},
  {...base,id:"checklist",format:"xlsx",name:l("研发与验收清单.xlsx","Development and acceptance.xlsx"),size:"128 KB",uploader:chen,time:"2026-10-08 10:12",purpose:l("跟踪文件卡片、格式预览与关联详情的开发和验收进度。","Track implementation and acceptance of cards, format previews and usage details."),source:cli,version:2},
  {...base,id:"architecture",format:"png",name:l("共享资料交互结构.png","Shared file interactions.png"),size:"842 KB",uploader:lin,time:"2026-10-08 10:26",purpose:l("展示三个入口如何复用同一份文件，以及预览与详情之间的关系。","Illustrate how three entry points use the same file, preview and details."),source:chat},
  {...base,id:"notes",format:"md",name:l("实现约定与接口笔记.md","Implementation notes.md"),size:"18 KB",uploader:zhou,time:"2026-10-08 10:00",purpose:l("记录版本引用、关联来源和文件类型渲染的实现约定，作为联调参考。","Record implementation rules for version references, associations and format rendering."),source:cli},
  {...base,id:"review",format:"pptx",name:l("设计评审汇报.pptx","Design review.pptx"),size:"4.1 MB",uploader:lin,time:"2026-10-08 11:02",purpose:l("向项目成员演示共享资料的设计方案，收集本轮评审意见。","Present the shared file design and collect review feedback."),source:web,task:false,usages:[{scope:"plan",actor:lin,time:"2026-10-08 11:05",note:l("用于协同体验优化计划的设计评审。","Used in the plan design review.")}]},
  {...base,id:"schema",format:"json",name:l("资料元信息示例.json","File metadata example.json"),size:"6 KB",uploader:chen,time:"2026-10-08 11:18",purpose:l("提供用途、上传人与计划任务关联字段的样例，供前后端核对。","Provide example fields for purpose, uploader and plan/task associations."),source:cli},
  {...base,id:"assets",format:"zip",name:l("设计素材与导出文件.zip","Design assets and exports.zip"),size:"8.6 MB",uploader:lin,time:"2026-10-08 11:30",purpose:l("汇总本轮设计素材、文档和截图，方便开发成员下载使用。","Collect design assets, documents and screenshots for developers."),source:web,task:false,usages:[]},
 ];
 return materials.map(file=>{
  const usages=file.id==="brief"||!file.task?[...file.usages]:[{scope:"task" as const,actor:file.uploader,time:file.time,note:file.purpose}];
  for(const message of seedMessages().filter(m=>m.fileIds.includes(file.id))){
   const time="2026-10-08 "+message.time;
   if(!usages.some(v=>v.scope===message.scope&&v.time===time&&v.actor.zh===message.actor.zh))usages.push({scope:message.scope,actor:message.actor,time,note:l("在讨论中分享，供当前工作参考。","Shared in the discussion as a reference for this work.")});
  }
  return {...file,usages};
 });
}
export const seedMessages=():Message[]=>[
 {id:"m1",actor:zhou,time:"10:05",text:l("我把共享资料的体验方案整理好了。资料库、讨论右侧和消息里的文件，建议使用同一套卡片与预览。","I’ve put together the shared file proposal. The library, discussion panel and messages can use the same card and preview."),fileIds:["brief","notes"],scope:"plan"},
 {id:"m2",actor:chen,time:"10:18",text:l("可以。详情需要能看清材料的用途，以及是谁在什么时间把它关联到哪个计划、哪个任务。","Agreed. Details should show the purpose and who associated the file with a plan or task, and when."),fileIds:[],scope:"plan"},
 {id:"m3",actor:lin,time:"10:28",text:l("这张图补充了三个入口之间的关系。点文件封面预览，点“详情”看用途和关联记录，操作保持一致。","This diagram shows the relationship between the three entry points. Click the cover to preview, or Details to see purpose and history."),fileIds:["architecture"],scope:"task"},
 {id:"m4",actor:l("Judex","Judex"),time:"10:30",text:l("已整理本次讨论材料。可结合体验方案和接口笔记核对实现范围；具体设计仍由参与者评审确认。","The discussion materials are organised. Review the proposal and implementation notes to confirm scope; the participants make the design decision."),fileIds:[],scope:"plan",ai:true},
];
export const sheetRows=[
 ["文件卡片","File cards","林晓","Lin Xiao","已完成","Complete",100],
 ["用途与关联详情","Purpose & associations","周屿","Zhou Yu","已完成","Complete",100],
 ["多格式内容预览","Format previews","陈沐","Chen Mu","进行中","In progress",75],
 ["讨论附件复用","Discussion attachments","周屿","Zhou Yu","进行中","In progress",60],
 ["桌面交互验收","Desktop acceptance","陈沐","Chen Mu","待开始","Planned",0],
] as const;
export function formatFor(name:string):Format{
 const ext=name.split(".").pop()?.toLowerCase();
 if(["png","jpg","jpeg","gif","webp","svg"].includes(ext??""))return "png";
 if(["md","txt","csv","log","yaml","yml"].includes(ext??""))return "md";
 return (["pdf","docx","xlsx","pptx","json","zip"].includes(ext??"")?ext:"other") as Format;
}
export const sizeOf=(size:number)=>size>=1024*1024?`${(size/1024/1024).toFixed(1)} MB`:`${Math.max(1,Math.round(size/1024))} KB`;
