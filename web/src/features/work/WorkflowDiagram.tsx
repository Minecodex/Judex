import {useEffect,useRef,useState,type CSSProperties} from 'react';
import {Minus,Plus,Scan,Maximize2,ArrowLeftRight,ArrowUpDown} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {useWork} from './store';
import {uid} from './seed';
import {workflowCode,type WorkflowGraph as Diagram} from './workflowGraph';
export {workflowCode} from './workflowGraph';
type Size={width:number;height:number};
// Mermaid has global configuration. Serialize rendering so simultaneous
// thumbnail and full-screen requests cannot overwrite each other's theme.
let queue: Promise<unknown> = Promise.resolve();
export function WorkflowDiagram({flow,thumbnail=false,natural=false,direction='LR',onReady}: {flow:Diagram;thumbnail?:boolean;natural?:boolean;direction?:'LR'|'TB';onReady?:(size:Size)=>void}) {
  const {locale,theme,t} = useWork();
  const ref = useRef<HTMLDivElement>(null);
  const [svg,setSVG] = useState(''), [error,setError] = useState(false), [attempt,retry] = useState(0);
  const callback = useRef(onReady);callback.current=onReady;
  const code = workflowCode(flow,locale === 'en',direction);
  useEffect(() => {
    let live = true;setSVG('');setError(false);
    const render = async () => {
      if (!live || !ref.current) return;
      const {default:mermaid} = await import('mermaid');
      if (!live || !ref.current) return;
      const style = getComputedStyle(ref.current);
      const color = (token:string) => {
        const context = document.createElement('canvas').getContext('2d')!;
        context.fillStyle = style.getPropertyValue(token).trim();context.fillRect(0,0,1,1);
        const [r,g,b] = context.getImageData(0,0,1,1).data;return 'rgb('+r+','+g+','+b+')';
      };
      mermaid.initialize({startOnLoad:false,securityLevel:'strict',theme:'base',
        themeVariables:{fontFamily:style.fontFamily,fontSize:style.getPropertyValue('--text-base').trim(),
          primaryColor:color('--accent-soft'),primaryTextColor:color('--ink'),primaryBorderColor:color('--line'),lineColor:color('--muted'),
          clusterBkg:color('--surface-soft'),clusterBorder:color('--line')}});
      const result = await mermaid.render('judexFlow'+uid().replaceAll('-',''),code);
      if (live) {
        setSVG(result.svg);
        const viewbox=new DOMParser().parseFromString(result.svg,'image/svg+xml').documentElement.getAttribute('viewBox')?.split(/\s+/).map(Number);
        if(viewbox && viewbox.length===4 && viewbox[2]>0 && viewbox[3]>0)callback.current?.({width:viewbox[2],height:viewbox[3]});
      }
    };
    queue = queue.catch(() => {}).then(render).catch(() => {if(live)setError(true);});
    return () => {live=false;};
  },[code,theme,attempt]);
  return <div ref={ref} className={'judex-work-flow-figure'+(thumbnail?' judex-workflow-thumbnail':'')+(natural?' judex-workflow-natural-figure':'')} data-testid="workflow-diagram" aria-label={t('workFlows')}>
    {error ? <div role="alert"><p>{t('workflowDiagramFailed')}</p><Button size="sm" onPress={() => retry(attempt+1)}>{t('workflowDiagramRetry')}</Button></div> :
      svg ? <div dangerouslySetInnerHTML={{__html:svg}}/> : <span role="status">{t('workflowDiagramLoading')}</span>}
  </div>;
}
export function WorkflowViewer({flow}: {flow:Diagram}) {
  const {t} = useWork();
  const [zoom,setZoom] = useState<number|null>(null), [pan,setPan] = useState({x:0,y:0}),[direction,setDirection]=useState<'LR'|'TB'>('LR');
  const [graphSize,setGraphSize]=useState<Size>({width:1,height:1}),[canvasSize,setCanvasSize]=useState<Size>({width:1,height:1});
  const viewport=useRef<HTMLDivElement>(null);
  const drag = useRef<{x:number;y:number;panX:number;panY:number}|null>(null);
  useEffect(()=>{
    if(!viewport.current)return;
    const observer=new ResizeObserver(entries=>{const size=entries[0]?.contentRect;if(size)setCanvasSize({width:size.width,height:size.height});});
    observer.observe(viewport.current);return ()=>observer.disconnect();
  },[]);
  const fit=Math.min(1,canvasSize.width/graphSize.width,canvasSize.height/graphSize.height),scale=zoom ?? fit;
  const reset=()=>{setZoom(null);setPan({x:0,y:0});};
  const style = {'--diagram-scale':scale,'--diagram-pan-x':pan.x+'px','--diagram-pan-y':pan.y+'px',
    '--diagram-width':graphSize.width+'px','--diagram-height':graphSize.height+'px'} as CSSProperties;
  return <div className="judex-workflow-viewer">
    <div className="judex-workflow-viewer-toolbar">
      <Button size="sm" isIconOnly aria-label={t('workflowDiagramZoomOut')} onPress={() => setZoom(Math.max(fit/4,scale/1.25))}><Minus/></Button>
      <Button size="sm" onPress={reset}><Scan/>{t('workflowDiagramFit')}</Button>
      <Button size="sm" isIconOnly aria-label={t('workflowDiagramZoomIn')} onPress={() => setZoom(Math.min(4,scale*1.25))}><Plus/></Button>
      <Button size="sm" data-testid="workflow-natural-size" onPress={() => {setZoom(1);setPan({x:0,y:0});}}><Maximize2/>{t('workflowDiagramNatural')}</Button>
      <Button size="sm" onPress={() => {setDirection(value=>value==='LR'?'TB':'LR');reset();}}>{direction==='LR'?<ArrowUpDown/>:<ArrowLeftRight/>}{t(direction==='LR'?'workflowDiagramVertical':'workflowDiagramHorizontal')}</Button>
      <span>{Math.round(scale*100)}%</span><small>{t('workflowDiagramPan')}</small>
    </div>
    <div ref={viewport} className="judex-workflow-viewport" data-testid="workflow-preview-viewport" style={style}
      onPointerDown={event => {drag.current={x:event.clientX,y:event.clientY,panX:pan.x,panY:pan.y};event.currentTarget.setPointerCapture(event.pointerId);}}
      onPointerMove={event => {if(drag.current)setPan({x:drag.current.panX+event.clientX-drag.current.x,y:drag.current.panY+event.clientY-drag.current.y});}}
      onPointerUp={() => {drag.current=null;}} onPointerCancel={() => {drag.current=null;}}>
      <WorkflowDiagram flow={flow} natural direction={direction} onReady={setGraphSize}/>
    </div>
    <p className="judex-work-small-note">{t('workflowDiagramLegend')}</p>
  </div>;
}
