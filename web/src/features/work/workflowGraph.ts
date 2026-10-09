import type {Flow} from './types.ts';
export type WorkflowGraph = Pick<Flow,'nodes'|'edges'|'connections'>;
export function workflowCode(flow: WorkflowGraph, english:boolean, direction:'LR'|'TB'='LR') {
  const language=english?'en':'zh', label=(value:string)=>value.replace(/["<>\r\n]/g,' ');
  const ids=new Map(flow.nodes.map((node,index)=>[node.id,'node'+index]));
  const groups=new Map<string,typeof flow.nodes>();
  for(const node of flow.nodes){
    const phase=node.phase?.[language] ?? '';
    groups.set(phase,[...groups.get(phase) ?? [],node]);
  }
  const lines=['flowchart '+direction];
  let phaseIndex=0;
  for(const [phase,nodes] of groups){
    if(phase)lines.push('  subgraph phase'+phaseIndex+++'["'+label(phase)+'"]');
    for(const node of nodes){
      const content='"'+label(node.label[language])+'"';
      lines.push('  '+ids.get(node.id)+(node.kind==='decision'?'{'+content+'}':'['+content+']'));
    }
    if(phase)lines.push('  end');
  }
  const edges=flow.connections ?? flow.edges.map(([from,to])=>({from,to,kind:'sequence' as const,label:undefined}));
  for(const edge of edges){
    const caption=edge.label?.[language];
    if(edge.kind==='feedback')lines.push('  '+ids.get(edge.from)+' -. "'+label(caption ?? '')+'" .-> '+ids.get(edge.to));
    else lines.push('  '+ids.get(edge.from)+(caption?' -->|"'+label(caption)+'"| ':' --> ')+ids.get(edge.to));
  }
  return lines.join('\n');
}
