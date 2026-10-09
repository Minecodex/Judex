import {createContext,useContext,type ReactNode} from 'react';
import type {TaskRoutePreviewKey} from '../src/i18n/taskRoutePreview';
import type {ModalState,Page,Section,State,Task,Text} from './model';
import type {Action,Result} from './actions';
export type GraphView={zoom:number;left:number;top:number};
export type Preview={state:State;setState:(update:(s:State)=>State)=>void;page:Page;go:(p:Page)=>void;t:(key:TaskRoutePreviewKey,values?:Record<string,string|number>)=>string;text:(v:Text|string)=>string;locale:'zh-CN'|'en';modal:ModalState|null;setModal:(m:ModalState|null)=>void;notify:(message:string)=>void;drawer:{id:string;section:Section}|null;setDrawer:(v:{id:string;section:Section}|null)=>void;viewer:{planId:string;preview:boolean;focus?:string}|null;openViewer:(planId:string,preview?:boolean,focus?:string)=>void;closeViewer:()=>void;views:Record<string,GraphView>;setView:(id:string,v:Partial<GraphView>)=>void;discuss:(task:Task)=>void};
export type PreviewOps={perform:(action:Action)=>Result};
const C=createContext<(Preview&PreviewOps)|null>(null);
export function Provider({value,children}:{value:Preview&PreviewOps;children:ReactNode}){return <C.Provider value={value}>{children}</C.Provider>;}
export function usePreview(){const value=useContext(C);if(!value)throw new Error('Demo4 context missing');return value;}
