import {renderDiagram} from '../../components/readers/diagram';
import {useEffect,useId,useRef,useState} from 'react';
import {Maximize2,Minus,Plus} from 'lucide-react';
import {useWorkspace} from './context';
import {Button,Empty,IconButton} from './ui';
import {graphSource,type WorkflowGraph} from './workflowGraph';
// Mermaid has a global configuration. Serialize initialization and rendering so
// multiple previews and theme changes cannot cross-contaminate each other.
let renderQueue=Promise.resolve();
export function WorkflowDiagram({graph,onSelect}:{graph:WorkflowGraph;onSelect?:(id:string)=>void}){
 const w=useWorkspace(),id=useId().replaceAll(':','').replaceAll('«','').replaceAll('»',''),host=useRef<HTMLDivElement>(null),viewport=useRef<HTMLDivElement>(null),[size,setSize]=useState({width:600,height:400}),select=useRef(onSelect),[svg,setSVG]=useState(''),[error,setError]=useState(false),[scale,setScale]=useState(1);
 select.current=onSelect;
 const source=graphSource(graph);
 const fit=()=>{const el=viewport.current;if(el){const p=getComputedStyle(el);setScale(Math.min(1,(el.clientWidth-parseFloat(p.paddingLeft)-parseFloat(p.paddingRight))/size.width,(el.clientHeight-parseFloat(p.paddingTop)-parseFloat(p.paddingBottom))/size.height));}};
 useEffect(()=>{let live=true;setError(false);setSVG('');if(!graph.nodes.length)return;const timer=setTimeout(()=>{renderQueue=renderQueue.catch(()=>{}).then(async()=>{if(!live)return;try{const out=await renderDiagram('judexgraph'+id,source);if(live){const element=new DOMParser().parseFromString(out.svg,'image/svg+xml').documentElement;const box=(element.getAttribute('viewBox')||'0 0 600 400').split(/[ ,]+/).map(Number);setSize({width:box[2],height:box[3]});setSVG(out.svg);}}catch{if(live)setError(true);}});},180);return()=>{live=false;clearTimeout(timer);};},[source,w.theme,id]);
 useEffect(()=>{const root=host.current;if(!root||!onSelect)return;const cleanups:(()=>void)[]=[];root.querySelectorAll<SVGGElement>('g.node').forEach(el=>{const match=el.id.match(/flowchart-n(\d+)-/),node=match&&graph.nodes[Number(match[1])];if(!node)return;el.setAttribute('role','button');el.setAttribute('tabindex','0');el.setAttribute('aria-label',node.name);const click=()=>select.current?.(node.id);const key=(event:KeyboardEvent)=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();click();}};el.addEventListener('click',click);el.addEventListener('keydown',key);cleanups.push(()=>{el.removeEventListener('click',click);el.removeEventListener('keydown',key);});});return()=>cleanups.forEach(f=>f());},[svg,onSelect,graph]);
 if(!graph.nodes.length)return <div className="judex-workflow-empty"><Empty title={w.t('wfEmpty')} description={w.t('wfEmptyHint')}/></div>;
 return <section className="judex-workflow-diagram" aria-label={w.t('wfDiagram')}><header><span>{w.t('wfCounts',{nodes:graph.nodes.length,edges:graph.edges.length})}</span><div><IconButton label={w.t('wfZoomOut')} isDisabled={scale<=.1} onPress={()=>setScale(v=>Math.max(.1,v-.25))}><Minus/></IconButton><Button onPress={()=>setScale(1)}>{Math.round(scale*100)}%</Button><IconButton label={w.t('wfZoomIn')} isDisabled={scale>=3} onPress={()=>setScale(v=>Math.min(3,v+.25))}><Plus/></IconButton><IconButton label={w.t('wfFit')} onPress={fit}><Maximize2/></IconButton></div></header><div ref={viewport} className="judex-workflow-viewport">{error?<p role="alert">{w.t('wfRenderFailed')}</p>:!svg?<p role="status">{w.t('v2Loading')}</p>:<div ref={host} className="judex-workflow-svg" style={{width:`${size.width*scale}px`}} dangerouslySetInnerHTML={{__html:svg}}/>}</div></section>;
}
