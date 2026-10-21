import {Fragment,useEffect,useRef,useState,type PointerEvent,type ReactNode} from 'react';
import {MessagesSquare,MessageSquareText,Files} from 'lucide-react';
import {useWorkspace} from './context';
import {IconButton} from './ui';
import {arrange,remember,defaultLayout,resizePair,useLayoutStore,type Layout,type Pane} from './layoutStore';
import '../../styles/workspace-panes.css';
const panes:Pane[]=['list','chat','reader'];
const paneIcons={list:MessagesSquare,chat:MessageSquareText,reader:Files};
function PaneToggle({pane,label,selected,onPress}:{pane:Pane;label:string;selected:boolean;onPress:()=>void}){
 const Icon=paneIcons[pane];
 return <IconButton className="judex-pane-toggle" label={label} aria-pressed={selected} data-pane-target={pane} onPress={onPress}><Icon/></IconButton>;
}
function useLayout(){const w=useWorkspace(),saved=useLayoutStore(s=>s.users[w.user.id]),save=useLayoutStore(s=>s.save);return {w,layout:saved||defaultLayout,save:(value:Layout)=>save(w.user.id,value)};}
function visiblePanes(layout:Layout,hasReader:boolean,expanded:boolean):Pane[]{
 const visible=panes.filter(p=>expanded&&hasReader?p==='reader':!layout.hidden[p]&&(p!=='reader'||hasReader));
 return visible.length?visible:['chat'];
}
/** A hidden pane's restore control lives in its nearest visible neighbour's header. */
export function PaneControls({pane}:{pane:Pane}){
 const {w,layout,save}=useLayout(),hasReader=!!w.reader||w.modal?.name==='library';
 const visible=visiblePanes(layout,hasReader,w.expanded);
 const name=(p:Pane)=>w.t(p==='list'?(w.screen==='chats'?'readerConversationList':'vpGroups'):p==='chat'?'vpChats':'vpPreview');
 const restore=panes.filter(p=>{
  if(visible.includes(p)||p==='reader'&&(!w.roomId||!hasReader&&pane==='chat'))return false;
  const nearest=[...visible].sort((a,b)=>Math.abs(panes.indexOf(a)-panes.indexOf(p))-Math.abs(panes.indexOf(b)-panes.indexOf(p)))[0];
  return nearest===pane;
 });
 const show=(p:Pane)=>{w.setExpanded(false);save({...layout,hidden:{...layout.hidden,[p]:false}});if(p==='reader'&&!hasReader)w.setModal({name:'library'});};
 const hide=()=>{w.setExpanded(false);const hidden={...layout.hidden,[pane]:true};if(panes.every(p=>hidden[p]||(p==='reader'&&!hasReader)))hidden[pane==='chat'?'list':'chat']=false;save({...layout,hidden});};
 return <div className="judex-pane-controls" data-pane-controls={pane}>
 {restore.map(p=><PaneToggle key={p} pane={p} selected={false} label={w.t('readerShowPane',{name:name(p)})} onPress={()=>show(p)}/>)}
 <PaneToggle pane={pane} selected={true} label={pane==='reader'?w.t('vpClosePreview'):w.t('readerHidePane',{name:name(pane)})} onPress={hide}/>
 </div>;
}
export function PaneLayout({list,chat,reader}:{list:ReactNode;chat:ReactNode;reader:ReactNode}){
 const {w,layout,save}=useLayout(),root=useRef<HTMLDivElement>(null),[live,setLive]=useState<Layout>(),[collapse,setCollapse]=useState<Pane>();
 const drag=useRef<{start:number;layout:Layout;left:Pane;right:Pane;visible:Pane[];width:number;minimum:Record<Pane,number>;result?:ReturnType<typeof resizePair>}|undefined>(undefined);
 const hasReader=!!w.reader||w.modal?.name==='library';
 useEffect(()=>{if(hasReader&&layout.hidden.reader)save({...layout,hidden:{...layout.hidden,reader:false}});},[w.reader?.id,w.modal?.name==='library']);
 const visible=visiblePanes(layout,hasReader,w.expanded);
 const effective=arrange(layout,visible),state=live||effective;
 const start=(event:PointerEvent<HTMLDivElement>,left:Pane,right:Pane)=>{if(event.button!==0||!root.current)return;event.preventDefault();event.currentTarget.setPointerCapture(event.pointerId);const unit=parseFloat(getComputedStyle(document.documentElement).fontSize);drag.current={start:event.clientX,layout:effective,left,right,visible,width:root.current.clientWidth,minimum:{list:16*unit,chat:22*unit,reader:20*unit}};};
 const move=(event:PointerEvent<HTMLDivElement>)=>{const d=drag.current;if(!d)return;d.result=resizePair(d.layout,d.left,d.right,event.clientX-d.start,d.width,d.visible,d.minimum);setLive(d.result.layout);setCollapse(d.result.collapse);};
 const finish=()=>{const d=drag.current;if(d?.result){const next=remember(layout,d.result.layout,d.visible);save(d.result.collapse?{...next,hidden:{...next.hidden,[d.result.collapse]:true}}:next);}drag.current=undefined;setLive(undefined);setCollapse(undefined);};
 const content={list,chat,reader};
 return <div ref={root} className={'judex-panes'+(live?' judex-panes-dragging':'')} data-layout={visible.join('-')}>
 {panes.map(p=>{const index=visible.indexOf(p),next=visible[index+1];return <Fragment key={p}><div className={'judex-pane judex-pane-'+p+(collapse===p?' judex-pane-collapse-preview':'')} hidden={index<0} style={{flexGrow:state.weights[p]}} data-pane={p}>{content[p]}</div>{index>=0&&next&&<div role="separator" tabIndex={0} aria-orientation="vertical" aria-label={w.t('readerResize')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(state.weights[p]/visible.reduce((n,v)=>n+state.weights[v],0)*100)} className="judex-pane-divider" onPointerDown={e=>start(e,p,next)} onPointerMove={move} onPointerUp={finish} onPointerCancel={()=>{drag.current=undefined;setLive(undefined);setCollapse(undefined);}} onDoubleClick={()=>save(defaultLayout)} onKeyDown={e=>{if(e.key==='ArrowLeft'||e.key==='ArrowRight'){e.preventDefault();const unit=parseFloat(getComputedStyle(document.documentElement).fontSize),r=resizePair(effective,p,next,(e.key==='ArrowLeft'?-1:1)*unit*2,root.current?.clientWidth||1200,visible,{list:16*unit,chat:22*unit,reader:20*unit});save(remember(layout,r.layout,visible));}if(e.key==='Home'){e.preventDefault();save(defaultLayout);}}}/>}</Fragment>;})}
 </div>;
}
