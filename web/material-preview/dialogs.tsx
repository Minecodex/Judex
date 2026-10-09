import {useState} from "react";
import {Check,ChevronLeft,ChevronRight,Compass,Download,FileText,MessageCircle,Paperclip,ShieldCheck,Upload,ZoomIn,ZoomOut} from "lucide-react";
import {Button} from "../src/components/ui/Button";
import {UIDialog,PersonAvatar} from "../src/components/ui/Presentation";
import {FormField,UIFilePicker,UIInput,UIOption,UISelect,UITextArea,UICheckbox} from "../src/components/ui/FormControls";
import {FileThumbnail,formatIcon} from "./cards";
import {FileContent,pageCount} from "./content";
import {usePreview,type Dialog} from "./context";
import {formatFor,l,sizeOf,type Material} from "./data";
export function MaterialDetails({file}:{file:Material}){
 const {t,text,setMode,open}=usePreview();
 return <div className="judex-mp-details" data-testid="material-details">
  <section className="judex-mp-purpose"><div><Compass/><h2>{t("mpPurpose")}</h2></div><p>{text(file.purpose)}</p></section>
  <div className="judex-mp-detail-columns"><section><h3>{t("mpInformation")}</h3><dl className="judex-mp-information">
   {[[t("mpUploader"),text(file.uploader)],[t("mpUploaded"),file.time],[t("mpSource"),text(file.source)],[t("mpVersion"),"v"+file.version],[t("mpFormat"),file.mime||file.format.toUpperCase()],[t("mpSize"),file.size],[t("mpVisibility"),t("mpProjectShared")]].map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
  </dl><div className="judex-mp-project-context"><small>{t("mpProjectLabel")}</small><strong>{t("mpProject")}</strong>{file.usages.length>0&&<><small>{t("mpPlanLabel")}</small><Button size="sm" variant="outline" onPress={()=>{open(null);setMode("plan");}}>{t("mpPlanName")}<MessageCircle/></Button></>}{file.task&&<><small>{t("mpTaskLabel")}</small><Button size="sm" variant="outline" onPress={()=>{open(null);setMode("task");}}>{t("mpTaskName")}<MessageCircle/></Button></>}</div></section>
   <section><h3>{t("mpUsage")}</h3><div className="judex-mp-timeline"><div className="judex-mp-timeline-item"><span className="judex-mp-timeline-dot"><Upload/></span><div><strong>{t("mpRecord")}</strong><p>{text(file.uploader)} · {text(file.source)}</p><time>{file.time}</time></div></div>
   {[...file.usages].sort((a,b)=>a.time.localeCompare(b.time)).map((usage,i)=><div className="judex-mp-timeline-item" key={i}><span className="judex-mp-timeline-dot"><Paperclip/></span><div><span className="judex-mp-scope-tag">{t(usage.scope==="task"?"mpTaskLabel":"mpPlanLabel")}</span><strong>{t(usage.scope==="task"?"mpTaskName":"mpPlanName")}</strong><p>{text(usage.note)}</p><span>{t("mpBy",{person:text(usage.actor)})}</span><time>{usage.time}</time></div></div>)}
   {!file.usages.length&&<p className="judex-mp-muted">{t("mpNoUsage")}</p>}
   </div></section></div>
 </div>;
}
function FileDialog({file,initial}:{file:Material;initial:"preview"|"details"}){
 const {t,text,open,setMode,mode,pending,setPending}=usePreview(),[tab,setTab]=useState(initial),[page,setPage]=useState(1),[zoom,setZoom]=useState(100),Icon=formatIcon(file.format);
 const share=()=>{setPending([...new Set([...pending,file.id])]);open(null);setMode(mode==="library"?"plan":mode);};
 const download=()=>{if(!file.url)return;const link=document.createElement("a");link.href=file.url;link.download=text(file.name);link.click();};
 return <UIDialog wide title={text(file.name)} description={`${file.format.toUpperCase()} · ${file.size} · v${file.version}`} onClose={()=>open(null)} footer={<>{file.local&&!file.deleted&&<Button size="sm" variant="outline" onPress={download}><Download/>{t("mpDownload")}</Button>}{!file.deleted&&<Button size="sm" variant="primary" onPress={share}><MessageCircle/>{t("mpSendFile")}</Button>}<Button size="sm" variant="outline" onPress={()=>open(null)}>{t("mpClose")}</Button></>}>
  <div className="judex-mp-file-dialog"><div className="judex-mp-modal-tabs"><Button aria-pressed={tab==="preview"} onPress={()=>setTab("preview")}><Icon/>{t("mpContent")}</Button><Button aria-pressed={tab==="details"} onPress={()=>setTab("details")}><Compass/>{t("mpDetailsTab")}</Button>{file.deleted?<span className="judex-mp-deleted-label">{t("mpDeleted")}</span>:!file.local&&<small>{t("mpExamplePreview")}</small>}</div>
  {tab==="details"?<MaterialDetails file={file}/>:file.deleted?<div className="judex-mp-unavailable"><FileText/><h3>{t("mpDeleted")}</h3><p>{t("mpDeletedHint")}</p></div>:<div className="judex-mp-reader-layout"><div className="judex-mp-reader"><div className="judex-mp-reader-toolbar"><span>{t("mpReading")}</span>{pageCount(file)>1&&<div><Button size="sm" isIconOnly aria-label={t("mpPrevious")} isDisabled={page===1} onPress={()=>setPage(page-1)}><ChevronLeft/></Button><small>{t("mpPage",{page,total:pageCount(file)})}</small><Button size="sm" isIconOnly aria-label={t("mpNext")} isDisabled={page===pageCount(file)} onPress={()=>setPage(page+1)}><ChevronRight/></Button></div>}{["pdf","docx","pptx","png","md"].includes(file.format)&&!(file.local&&file.format!=="png")&&<div><Button size="sm" isIconOnly aria-label={t("mpZoomOut")} isDisabled={zoom===75} onPress={()=>setZoom(Math.max(75,zoom-25))}><ZoomOut/></Button><small>{zoom}%</small><Button size="sm" isIconOnly aria-label={t("mpZoomIn")} isDisabled={zoom===150} onPress={()=>setZoom(Math.min(150,zoom+25))}><ZoomIn/></Button></div>}</div><div className="judex-mp-reader-canvas"><FileContent file={file} page={page} zoom={zoom}/></div></div><aside className="judex-mp-reader-sidebar"><span className="judex-mp-sidebar-label"><Compass/>{t("mpPurpose")}</span><p>{text(file.purpose)}</p><div className="judex-mp-reader-person"><PersonAvatar name={text(file.uploader)}/><div><strong>{text(file.uploader)}</strong><small>{file.time}</small></div></div><dl>{file.usages.length>0&&<><dt>{t("mpPlanLabel")}</dt><dd>{t("mpPlanName")}</dd></>}{file.task&&<><dt>{t("mpTaskLabel")}</dt><dd>{t("mpTaskName")}</dd></>}</dl><Button size="sm" variant="outline" onPress={()=>setTab("details")}>{t("mpUsage")}<ChevronRight/></Button><span className="judex-mp-sidebar-sharing"><ShieldCheck/>{t("mpProjectShared")}</span></aside></div>}
  </div>
 </UIDialog>;
}
function UploadDialog(){
 const p=usePreview(),{t}=p,[file,setFile]=useState<File|null>(null),[purpose,setPurpose]=useState(""),[scope,setScope]=useState(p.mode==="task"?"task":"plan"),[busy,setBusy]=useState(false),[error,setError]=useState("");
 const save=async()=>{
  if(!file||!purpose.trim()||busy)return;setBusy(true);setError("");
  try{const format=formatFor(file.name),isText=["md","json"].includes(format),content=isText?await file.text():undefined;
   const material:Material={id:"local-"+crypto.randomUUID(),name:l(file.name,file.name),format,size:sizeOf(file.size),uploader:l("林晓","Lin Xiao"),time:"2026-10-08 "+new Intl.DateTimeFormat("en-GB",{timeZone:"Asia/Shanghai",hour:"2-digit",minute:"2-digit"}).format(new Date()),purpose:l(purpose.trim(),purpose.trim()),source:l("网页 · 共享资料","Web · Shared files"),version:1,task:scope==="task",usages:[],local:true,url:URL.createObjectURL(file),mime:file.type,textContent:content};
   material.usages=[{scope:scope as "task"|"plan",actor:material.uploader,time:material.time,note:material.purpose}];p.upload(material);p.open(null);p.notify(t("mpUploadedNotice"));
  }catch{setError(t("mpReadError"));setBusy(false);}
 };
 return <UIDialog title={t("mpUploadTitle")} onClose={()=>p.open(null)} dismissable={!busy} footer={<><Button variant="outline" onPress={()=>p.open(null)} isDisabled={busy}>{t("mpCancel")}</Button><Button variant="primary" isDisabled={!file||!purpose.trim()||busy} onPress={save}><Upload/>{t("mpAddToLibrary")}</Button></>}>
  <p className="judex-mp-muted">{t("mpLocalHint")}</p><div className="judex-mp-upload-zone"><Upload/><UIFilePicker aria-label={t("mpSelectFile")} onChange={e=>setFile(e.target.files?.[0]??null)}>{t("mpSelectFile")}</UIFilePicker>{file&&<span>{file.name} · {sizeOf(file.size)}</span>}</div>
  <FormField label={t("mpUploadPurpose")}><UITextArea value={purpose} onChange={e=>setPurpose(e.target.value)} placeholder={t("mpUploadPlaceholder")}/></FormField>
  <FormField label={t("mpPlanLabel")}><UIInput value={t("mpPlanName")} readOnly/></FormField><FormField label={t("mpTaskLabel")}><UISelect value={scope} onChange={e=>setScope(e.target.value)}><UIOption value="plan">{t("mpNoTask")}</UIOption><UIOption value="task">{t("mpTaskName")}</UIOption></UISelect></FormField>{error&&<p role="alert">{error}</p>}
 </UIDialog>;
}
function ChooseDialog(){
 const p=usePreview(),{t,text}=p,[ids,setIds]=useState(p.pending);
 return <UIDialog wide title={t("mpChooseTitle")} description={t("mpChooseHint")} onClose={()=>p.open(null)} footer={<><span className="judex-mp-footer-count">{t("mpSelected",{count:ids.length})}</span><Button variant="outline" onPress={()=>p.open(null)}>{t("mpCancel")}</Button><Button variant="primary" onPress={()=>{p.setPending(ids);p.open(null);}}><Check/>{t("mpChooseConfirm")}</Button></>}>
  <div className="judex-mp-picker-grid">{p.files.filter(f=>!f.deleted).map(file=><UICheckbox key={file.id} aria-label={text(file.name)} checked={ids.includes(file.id)} onChange={e=>setIds(e.target.checked?[...ids,file.id]:ids.filter(v=>v!==file.id))} appearance="card"><div className="judex-mp-picker-card"><FileThumbnail file={file}/><strong>{text(file.name)}</strong><small>{text(file.uploader)} · {file.size}</small></div></UICheckbox>)}</div>
 </UIDialog>;
}
export function Dialogs({dialog}:{dialog:Dialog}){
 const p=usePreview(),file="id" in dialog?p.files.find(f=>f.id===dialog.id):undefined;
 if(dialog.kind==="upload")return <UploadDialog/>;
 if(dialog.kind==="choose")return <ChooseDialog/>;
 if(!file)return null;
 if(dialog.kind==="file")return <FileDialog key={file.id+dialog.tab} file={file} initial={dialog.tab}/>;
 const count=p.messages.filter(m=>m.fileIds.includes(file.id)).length;
 return <UIDialog title={p.t("mpDeleteTitle")} onClose={()=>p.open(null)} footer={<><Button variant="outline" onPress={()=>p.open(null)}>{p.t("mpCancel")}</Button><Button variant="danger" onPress={()=>{p.remove(file.id);p.open(null);p.notify(p.t("mpDeletedNotice"));}}>{p.t("mpConfirmDelete")}</Button></>}><strong>{p.text(file.name)}</strong>{count>0&&<p>{p.t("mpReferences",{count})}</p>}<p className="judex-mp-muted">{p.t("mpDeleteHint")}</p></UIDialog>;
}
