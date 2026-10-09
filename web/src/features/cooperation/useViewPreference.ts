import {useState} from "react";
import {useWork} from "../work/store";
// View filters never mutate business data or navigation. Actor/project keys
// preserve independent choices while moving between hub tabs and discussions.
export function useViewPreference<T>(slot:string,initial:T){
 const {state,project}=useWork(),key="judex.cooperation.view."+(state.currentUserId??state.currentUser)+":"+project.id+":"+slot;
 const read=()=>{try{return JSON.parse(sessionStorage.getItem(key)||"null")??initial;}catch{return initial;}};
 const [saved,setSaved]=useState(()=>({key,value:read() as T})),value=saved.key===key?saved.value:read() as T;
 const set=(next:T)=>{setSaved({key,value:next});try{sessionStorage.setItem(key,JSON.stringify(next));}catch{}};
 return [value,set] as const;
}
