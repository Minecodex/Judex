import {useEffect,useRef} from "react";

export function useScopedRequest(scope:string){
 const currentScope=useRef(scope),generation=useRef(0),controller=useRef<AbortController|undefined>(undefined);
 currentScope.current=scope;
 const invalidate=()=>{generation.current++;controller.current?.abort();controller.current=undefined;};
 useEffect(()=>{invalidate();return invalidate;},[scope]);
 const begin=()=>{
  invalidate();const id=generation.current,requestScope=scope,abort=new AbortController();controller.current=abort;
  return {signal:abort.signal,isCurrent:()=>generation.current===id&&currentScope.current===requestScope&&!abort.signal.aborted};
 };
 return {begin,invalidate};
}
