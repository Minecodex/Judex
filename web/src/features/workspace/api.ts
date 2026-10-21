import {request} from '../../lib/api/client';
export const list=async<T,>(path:string)=>(await request<{items:T[]}>(path)).items;
export const command=<T,>(path:string,body:unknown={},method='POST')=>request<T>(path,{method,body:JSON.stringify(body)});
