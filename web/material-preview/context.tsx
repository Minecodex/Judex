import {createContext,useContext} from "react";
import type {Locale} from "../src/i18n";
import type {MaterialPreviewKey} from "../src/i18n/materialPreview";
import type {Localized,Material,Message} from "./data";
export type Mode="library"|"plan"|"task";
export type Dialog={kind:"file";id:string;tab:"preview"|"details"}|{kind:"delete";id:string}|{kind:"upload"}|{kind:"choose"};
export type Preview={
 locale:Locale;mode:Mode;files:Material[];messages:Message[];pending:string[];setPending:(ids:string[])=>void;
 t:(key:MaterialPreviewKey,values?:Record<string,string|number>)=>string;text:(value:Localized)=>string;
 setMode:(mode:Mode)=>void;open:(dialog:Dialog|null)=>void;notify:(text:string)=>void;remove:(id:string)=>void;upload:(file:Material)=>void;
};
export const PreviewContext=createContext<Preview|null>(null);
export const usePreview=()=>useContext(PreviewContext)!;
