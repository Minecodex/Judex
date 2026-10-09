import {Card} from "@heroui/react";
import {Archive,Braces,Eye,FileImage,FileSpreadsheet,FileText,Info,Presentation,Trash2} from "lucide-react";
import {Button} from "../src/components/ui/Button";
import {PersonAvatar} from "../src/components/ui/Presentation";
import {usePreview} from "./context";
import type {Format,Material} from "./data";
export const formatIcon=(format:Format)=>format==="xlsx"?FileSpreadsheet:format==="png"?FileImage:format==="pptx"?Presentation:format==="json"?Braces:format==="zip"?Archive:FileText;
export function Architecture({mini=false}:{mini?:boolean}){
 const {locale}=usePreview(),en=locale==="en";
 return <svg className={"judex-mp-architecture"+(mini?" judex-mp-architecture-mini":"")} viewBox="0 0 720 440" role="img" aria-label={en?"Shared file interaction diagram":"共享资料交互结构图"}>
  <rect width="720" height="440" rx="18" fill="var(--mp-art-bg)"/>
  <text x="40" y="51" fontSize="23" fontWeight="600" fill="var(--mp-art-ink)">{en?"One file. Every conversation.":"同一份资料，贯穿每次讨论。"}</text>
  <text x="40" y="79" fontSize="13" fill="var(--mp-art-muted)">{en?"Shared context, consistent interactions":"项目共享 · 用途可追溯 · 交互一致"}</text>
  {[{x:40,label:en?"Project library":"项目资料库",n:"01"},{x:270,label:en?"Discussion panel":"讨论资料栏",n:"02"},{x:500,label:en?"Message card":"会话文件卡片",n:"03"}].map(v=><g key={v.x}>
   <rect x={v.x} y="123" width="180" height="109" rx="12" fill="var(--mp-art-paper)" stroke="var(--mp-art-line)"/>
   <text x={v.x+18} y="153" fontSize="12" fill="var(--mp-art-muted)">{v.n}</text><text x={v.x+18} y="186" fontSize="17" fill="var(--mp-art-ink)">{v.label}</text>
   <path d={`M${v.x+90} 232V275H360V303`} stroke="var(--mp-art-accent)" strokeWidth="2" fill="none"/>
  </g>)}
  <rect x="229" y="303" width="262" height="87" rx="12" fill="var(--mp-art-accent)"/>
  <text x="360" y="338" fontSize="19" fill="var(--mp-art-paper)" textAnchor="middle">{en?"Shared file record":"统一文件记录"}</text>
  <text x="360" y="365" fontSize="13" fill="var(--mp-art-paper)" textAnchor="middle">{en?"Preview · Purpose · History":"内容预览 · 材料用途 · 关联记录"}</text>
 </svg>;
}
export function FileThumbnail({file}:{file:Material}){
 const {t,text,locale}=usePreview(),en=locale==="en",Icon=formatIcon(file.format);
 return <div className={`judex-mp-thumbnail judex-mp-format-${file.format}`} aria-hidden="true">
  <span className="judex-mp-format-pill">{file.format==="other"?"FILE":file.format.toUpperCase()}</span>
  {file.format==="png"?(file.local?<img src={file.url} alt=""/>:<Architecture mini/>):file.format==="zip"?<div className="judex-mp-folder-art"><Archive/><span>design-assets</span><small>8 files</small></div>:
   file.format==="xlsx"?<div className="judex-mp-sheet-art"><div><FileSpreadsheet/><span>{en?"Acceptance checklist":"研发与验收清单"}</span></div><div className="judex-mp-mini-table">{Array.from({length:24},(_,i)=><span key={i}>{i<4?["A","B","C","D"][i]:i%4===0?String(Math.ceil(i/4)):i%4===3?"✓":""}</span>)}</div><div className="judex-mp-mini-bars">{[72,100,48,84,56].map((v,i)=><i key={i} style={{height:v+"%"}}/>)}</div></div>:
   file.format==="json"||file.format==="md"?<div className="judex-mp-code-art"><span>{file.format==="json"?"{":"# "+(en?"Implementation notes":"实现约定")}</span>{(file.format==="json"?["  \"purpose\": \"design review\",","  \"project\": \"Judex\",","  \"version\": 1,","  \"associations\": [ … ]","}"]:["","## "+(en?"Shared file references":"统一资料引用"),"","- materialId + versionId","- preview / details","- preserve usage history"]).map((v,i)=><p key={i}>{v||" "}</p>)}</div>:
   file.format==="pptx"?<div className="judex-mp-slide-art"><small>JUDEX / DESIGN REVIEW</small><strong>{en?"Files, in context.":"让资料回到工作中。"}</strong><div><span>01</span><span>02</span><span>03</span></div></div>:
   <div className="judex-mp-paper-art"><Icon/><small>JUDEX · COLLABORATION</small><strong>{file.format==="pdf"?(en?"Shared file experience":"共享资料体验方案"):(en?"File preview requirements":"资料预览需求说明")}</strong><span className="judex-mp-paper-rule"/><p>{en?"Purpose, context, and a consistent preview.":"材料有用途，来源可追溯。"}</p>{[1,2,3,4].map(v=><i key={v}/>)}</div>}
  <span className="judex-mp-thumbnail-caption">{file.local?text(file.name):en?"Judex · Working materials":"Judex · 工作材料"}</span>
  <span className="judex-mp-preview-hover"><Eye/>{t("mpPreview")}</span>
 </div>;
}
export function FileCard({file,compact=false}:{file:Material;compact?:boolean}){
 const {t,text,open}=usePreview(),name=text(file.name);
 if(file.deleted)return <div className="judex-mp-deleted-reference" data-file-id={file.id}><FileText/><div><strong>{name}</strong><span>{t("mpDeleted")} · {t("mpDeletedHint")}</span></div><Button size="sm" isIconOnly aria-label={t("mpDetailsFile",{name})} onPress={()=>open({kind:"file",id:file.id,tab:"details"})}><Info/></Button></div>;
 return <Card className={"judex-mp-file-card"+(compact?" judex-mp-file-card-compact":"")} data-file-id={file.id} role="article" aria-label={name}>
  <Button className="judex-mp-cover-button" aria-label={t("mpPreviewFile",{name})} onPress={()=>open({kind:"file",id:file.id,tab:"preview"})}><FileThumbnail file={file}/></Button>
  <Card.Content className="judex-mp-card-content"><Button className="judex-mp-file-title" aria-label={t("mpPreviewFile",{name})} onPress={()=>open({kind:"file",id:file.id,tab:"preview"})}>{name}</Button>
   <div className="judex-mp-file-subtitle"><span>{file.size} · v{file.version}</span><span className="judex-mp-scope-tag">{t(file.task?"mpTaskLabel":file.usages.length?"mpPlanLabel":"mpProjectLabel")}</span></div>
   <div className="judex-mp-upload-meta"><PersonAvatar name={text(file.uploader)} small/><div><span>{text(file.uploader)}</span><time>{file.time}</time></div></div>
  </Card.Content>
  <Card.Footer className="judex-mp-card-footer"><Button size="sm" aria-label={t("mpDetailsFile",{name})} onPress={()=>open({kind:"file",id:file.id,tab:"details"})}><Info/>{t("mpDetails")}</Button><Button size="sm" className="judex-mp-delete-button" aria-label={t("mpDeleteFile",{name})} onPress={()=>open({kind:"delete",id:file.id})}><Trash2/>{t("mpDelete")}</Button></Card.Footer>
 </Card>;
}
