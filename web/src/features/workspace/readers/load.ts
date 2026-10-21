export const TEXT_LIMIT=4*1024*1024,WORKBOOK_LIMIT=20*1024*1024;
export async function loadBytes(url:string,limit:number,signal?:AbortSignal){
 const response=await fetch(url,{credentials:'same-origin',headers:{Range:`bytes=0-${limit-1}`},signal});
 if(response.status===416)return {bytes:new ArrayBuffer(0),partial:false};
 if(!response.ok)throw new Error('readerFailed');
 const reader=response.body?.getReader();if(!reader)throw new Error('readerFailed');const chunks:Uint8Array[]=[];let size=0;
 try{while(true){const {value,done}=await reader.read();if(done)break;size+=value.length;if(size>limit)throw new Error('readerSizeLimit');chunks.push(value);}}finally{await reader.cancel();}
 const bytes=new Uint8Array(size);let offset=0;for(const chunk of chunks){bytes.set(chunk,offset);offset+=chunk.length;}
 const total=Number(response.headers.get('content-range')?.split('/').at(-1)||response.headers.get('content-length')||size);
 return {bytes:bytes.buffer,partial:total>size};
}
