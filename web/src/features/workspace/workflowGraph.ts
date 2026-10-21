export type WorkflowNode={id:string;name:string;kind:'start'|'activity'|'decision'|'end';phase:string;responsibility:string};
export type WorkflowEdge={from:string;to:string;kind:'sequence'|'feedback';label:string};
export type WorkflowGraph={nodes:WorkflowNode[];edges:WorkflowEdge[]};
export type WorkflowDraft={name:string;content:string;graph:WorkflowGraph};
export const emptyGraph=():WorkflowGraph=>({nodes:[],edges:[]});
// Generate syntax exclusively from structured data. No user Mermaid directives,
// identifiers, links, HTML or callbacks are interpolated into the renderer.
const label=(value:string)=>value.replace(/[\n\r]/g,' ').replace(/["`\\|<>&\[\]{}]/g,c=>String.fromCharCode(c.charCodeAt(0)+0xfee0));
export function graphSource(graph:WorkflowGraph){
 const index=new Map(graph.nodes.map((n,i)=>[n.id,'n'+i]));
 const nodes=graph.nodes.map((n,i)=>{const text=`"${label(n.name)}"`;return `n${i}${n.kind==='decision'?'{'+text+'}':n.kind==='start'||n.kind==='end'?'(['+text+'])':'['+text+']'}`;});
 const edges=graph.edges.filter(e=>index.has(e.from)&&index.has(e.to)).map(e=>`${index.get(e.from)} ${e.kind==='feedback'?'-.->':'-->'}${e.label?'|"'+label(e.label)+'"|':''} ${index.get(e.to)}`);
 return ['flowchart TD',...nodes,...edges].join('\n');
}
