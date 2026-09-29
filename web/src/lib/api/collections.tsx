import {useInfiniteQuery,type QueryKey} from "@tanstack/react-query";
import {Button} from "@heroui/react";
import {request} from "./client";
import {usePreferences} from "../../stores/preferences";
import {translate} from "../../i18n";
export function useCollection<T>(key:QueryKey,url:string,enabled=true){
 const query=useInfiniteQuery({queryKey:[...key,"pages"],enabled,initialPageParam:"",queryFn:({pageParam})=>request<{items:T[];nextCursor:string|null}>(url+(url.includes("?")?"&":"?")+"limit=50"+(pageParam?"&cursor="+encodeURIComponent(pageParam):"")),getNextPageParam:(page)=>page.nextCursor||undefined});
 return {...query,data:query.data?{items:query.data.pages.flatMap((p)=>p.items??[])}:undefined};
}
export function LoadMore({query,label}:{query:{hasNextPage:boolean;isFetchingNextPage:boolean;fetchNextPage:()=>Promise<unknown>};label?:string}){
 const {locale}=usePreferences();if(!query.hasNextPage)return null;
 return <Button variant="ghost" isPending={query.isFetchingNextPage} onPress={()=>void query.fetchNextPage()}>{translate(locale,"lcLoadMore")}{label?" · "+label:""}</Button>;
}
