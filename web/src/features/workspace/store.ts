import {create} from 'zustand';
import {persist} from 'zustand/middleware';
import type {Mention} from './types';
export type ReplyTarget={id:string;authorName:string;body:string};
type Draft={body:string;mentions:Mention[];reply?:ReplyTarget;confirmationTitle?:string};
type Drafts={drafts:Record<string,Draft>;setDraft:(key:string,draft:Draft)=>void;clearUser:(user:string)=>void};
// Draft keys include the authenticated user; business resources never enter this store.
export const useDrafts=create<Drafts>()(persist(set=>({drafts:{},setDraft:(key,draft)=>set(s=>({drafts:{...s.drafts,[key]:draft}})),clearUser:user=>set(s=>({drafts:Object.fromEntries(Object.entries(s.drafts).filter(([key])=>!key.startsWith(user+':')))}))}),{name:'judex.v2.drafts'}));
