import {useEffect,useState,type SetStateAction} from "react";
import { useWork } from "../work/store";
export function useWorkspaceDraft<T>(slot:string,initial:T,basis?:string){
 const {project,state}=useWork();
 const key="judex.workspace.draft."+project.id+":"+(state.currentUserId??state.currentUser)+":"+slot;
 const base=basis??JSON.stringify(initial);
 const read=():T=>{try{const saved=JSON.parse(sessionStorage.getItem(key)||"null");return saved?.base===base?saved.value:initial;}catch{return initial;}};
 const [stored,setStored]=useState(()=>({key,base,value:read()}));
 const value=stored.key===key&&stored.base===base?stored.value:read();
 const setValue=(next:SetStateAction<T>)=>setStored(prev=>{const current=prev.key===key&&prev.base===base?prev.value:read();return {key,base,value:typeof next==="function"?(next as (v:T)=>T)(current):next};});
 const clear=()=>{sessionStorage.removeItem(key);setStored({key,base,value:initial});};
 useEffect(()=>{try{if(JSON.stringify(value)===JSON.stringify(initial))sessionStorage.removeItem(key);else sessionStorage.setItem(key,JSON.stringify({base,value}));}catch{}},[key,base,value]);
 return [value,setValue,clear] as const;
}
