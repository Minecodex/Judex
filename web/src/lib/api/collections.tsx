import {useInfiniteQuery,type QueryKey} from "@tanstack/react-query";
import {useEffect} from "react";
import {Button} from "../../components/ui/Button";
import {request} from "./client";
import {usePreferences} from "../../stores/preferences";
import {translate} from "../../i18n";
export function useCollection<T>(key:QueryKey,url:string,enabled=true){
 const query=useInfiniteQuery({queryKey:[...key,"pages"],enabled,refetchOnMount:'always',initialPageParam:"",queryFn:({pageParam})=>request<{items:T[];nextCursor:string|null;totalCount?:number}>(url+(url.includes("?")?"&":"?")+"limit=50"+(pageParam?"&cursor="+encodeURIComponent(pageParam):"")),getNextPageParam:(page)=>page.nextCursor||undefined});
 const storage="judex.collection.pages."+JSON.stringify(key);
 let desired=1;try{desired=Math.max(1,Math.min(100,Number(sessionStorage.getItem(storage))||1));}catch{}
 useEffect(()=>{if(!enabled||!query.data||query.isFetching||query.isError)return;const count=query.data.pages.length;if(count<desired&&query.hasNextPage){void query.fetchNextPage();return;}try{sessionStorage.setItem(storage,String(count));}catch{}},[storage,enabled,desired,query.data?.pages.length,query.isFetching,query.isError,query.hasNextPage]);
 return {...query,data:query.data?{items:query.data.pages.flatMap((p)=>p.items??[]),totalCount:query.data.pages[0]?.totalCount}:undefined};
}
export function LoadMore({query,label}:{query:{hasNextPage:boolean;isFetchingNextPage:boolean;fetchNextPage:()=>Promise<unknown>};label?:string}){
 const {locale}=usePreferences();if(!query.hasNextPage)return null;
 return <Button variant="ghost" isPending={query.isFetchingNextPage} onPress={()=>void query.fetchNextPage()}>{translate(locale,"lcLoadMore")}{label?" · "+label:""}</Button>;
}
