import {createContext,useContext,type ReactNode} from "react";
import type {CooperationKey} from "../src/i18n/cooperationPreview";
import type {Action,Result} from "./actions";
import type {Plan,Route,State,Task,Text,Topic} from "./model";
export type ModalState={kind:"route";planId:string}|{kind:"task";taskId:string}|{kind:"report";taskId:string;recordKind:"progress"|"delivery"|"question"|"reply";local?:boolean}|{kind:"decision";decisionId:string}|{kind:"receipt";handoffId:string}|{kind:"topic";parentId?:string;afterSeq?:number;topicId?:string}|{kind:"suggestion";suggestionId:string;mode:"create"|"existing"}|{kind:"settings";projectId:string;section?:string}|{kind:"invite";projectId:string}|{kind:"arrangement";sourceTopicId:string}|{kind:"project"}|{kind:"material";materialId:string}|{kind:"record";recordId:string};
export type Preview={state:State;route:Route;locale:"zh-CN"|"en";theme:"light"|"dark";t:(key:CooperationKey)=>string;text:(v:Text|string)=>string;person:(id:string)=>string;perform:(a:Action)=>Result;go:(r:Route)=>void;setModal:(v:ModalState|null)=>void;setLocale:(v:"zh-CN"|"en")=>void;setTheme:(v:"light"|"dark")=>void;planChat:(p:Plan)=>void;taskChat:(t:Task)=>void;topicChat:(topic:Topic,scope?:{planId?:string;taskId?:string;messageSeq?:number})=>void;notify:(value:string)=>void;reset:()=>void;logout:()=>void};
export const PreviewContext=createContext<Preview|null>(null);
export function usePreview(){const value=useContext(PreviewContext);if(!value)throw new Error("Preview context unavailable");return value;}
export function PreviewProvider({value,children}:{value:Preview;children:ReactNode}){return <PreviewContext.Provider value={value}>{children}</PreviewContext.Provider>;}
