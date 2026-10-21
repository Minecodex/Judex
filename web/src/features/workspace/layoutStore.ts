import {create} from 'zustand';
import {persist} from 'zustand/middleware';
export type Pane='list'|'chat'|'reader';
export type Layout={weights:Record<Pane,number>;hidden:Record<Pane,boolean>;arrangements?:Record<string,Record<Pane,number>>};
export const defaultLayout:Layout={weights:{list:22,chat:43,reader:35},hidden:{list:false,chat:false,reader:false}};
type Store={users:Record<string,Layout>;save:(user:string,layout:Layout)=>void;reset:(user:string)=>void};
export const useLayoutStore=create<Store>()(persist(set=>({users:{},save:(user,layout)=>set(s=>({users:{...s.users,[user]:layout}})),reset:user=>set(s=>({users:{...s.users,[user]:defaultLayout}}))}),{name:'judex.v2.layout'}));
export function resizePair(layout:Layout,left:Pane,right:Pane,delta:number,totalPixels:number,visible:Pane[],minimum:Record<Pane,number>):{layout:Layout;collapse?:Pane}{
 const sum=visible.reduce((n,p)=>n+layout.weights[p],0),factor=totalPixels/sum;
 const pair=layout.weights[left]+layout.weights[right],a=layout.weights[left]*factor+delta,b=pair*factor-a;
 const collapse=a<minimum[left]*.55?left:b<minimum[right]*.55?right:undefined;
 const lower=Math.min(minimum[left]/factor,pair*.4),upper=pair-Math.min(minimum[right]/factor,pair*.4);
 const weight=Math.max(lower,Math.min(upper,a/factor));
 return {layout:{...layout,weights:{...layout.weights,[left]:weight,[right]:pair-weight}},collapse};
}

// Each visible arrangement remembers its own widths. Closing the reader gives
// its space to the chat while preserving the three-pane arrangement for return.
export function arrange(layout:Layout,visible:Pane[]):Layout{
 if(visible.length!==2)return layout;
 const key=visible.join('-'),saved=layout.arrangements?.[key];
 if(saved)return {...layout,weights:saved};
 const weights={...layout.weights},receiver=visible.includes('chat')?'chat':visible[1];
 weights[receiver]=100-visible.filter(p=>p!==receiver).reduce((sum,p)=>sum+weights[p],0);
 return {...layout,weights};
}
export function remember(original:Layout,changed:Layout,visible:Pane[]):Layout{
 return visible.length===2?{...original,arrangements:{...original.arrangements,[visible.join('-')]:changed.weights}}:{...original,weights:changed.weights};
}
