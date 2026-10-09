import {useEffect,useState,type SetStateAction} from "react";
import {useWork} from "../work/store";
import type {Evidence} from "../work/types";

// Attachments are UI drafts kept only in this browser's memory. Refreshing
// releases them; navigating or closing a dialog preserves the owning draft.
const attachments=new Map<string,Evidence[]>();
export function useWorkspaceAttachments(slot:string){
 const {mode,project,state}=useWork(),key=JSON.stringify([mode,project.id,state.currentUserId??state.currentUser,slot]);
 const read=()=>attachments.get(key)??[];
 const [stored,setStored]=useState(()=>({key,files:read()})),files=stored.key===key?stored.files:read();
 const setFiles=(next:SetStateAction<Evidence[]>)=>setStored(previous=>{const current=previous.key===key?previous.files:read();return {key,files:typeof next==="function"?next(current):next};});
 useEffect(()=>{if(files.length)attachments.set(key,files);else attachments.delete(key);},[key,files]);
 const clear=()=>{attachments.delete(key);setStored({key,files:[]});};
 return [files,setFiles,clear] as const;
}
