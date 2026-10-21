import {lazy,Suspense,useState} from 'react';
import {ArrowLeft,FileText} from 'lucide-react';
import {useWorkspace} from '../context';
import {Button,Empty} from '../ui';
import {PdfReader} from '../../../components/readers/PdfReader';
import {ImageReader,MediaReader} from './VisualReaders';
import {readerKind} from './formats';
import type {FileVersion} from '../types';
import {apiUrl} from '../../../lib/api/client';
const TableReader=lazy(()=>import('./TableReader')),TextReader=lazy(()=>import('./TextReader'));
export function ReaderSurface({file,url,original,html,entry=false}:{file:FileVersion;url:string;original:string;html?:React.ReactNode;entry?:boolean}){
 const w=useWorkspace(),kind=readerKind(file.name,file.previewData?.kind),[mode,setMode]=useState('table'),[selected,setSelected]=useState<{path:string;size:number;mime:string}>();
 let content:React.ReactNode;
 if(kind==='workbook')content=<><div className="judex-reader-toolbar"><Button aria-pressed={mode==='table'} onPress={()=>setMode('table')}>{w.t('readerTable')}</Button>{!entry&&<Button aria-pressed={mode==='pdf'} onPress={()=>setMode('pdf')}>{w.t('readerPages')}</Button>}</div>{mode==='pdf'?<PdfReader url={url}/>:<TableReader url={original} size={file.size} name={file.name} workbook/>}</>;
 else if(kind==='table')content=<TableReader url={original} size={file.size} name={file.name}/>;
 else if(kind==='text'||kind==='markdown'||kind==='diagram')content=<TextReader url={original} name={file.name} kind={kind}/>;
 else if(kind==='pdf'&&(!entry||/\.pdf$/i.test(file.name)))content=<PdfReader url={url}/>;
 else if(kind==='image')content=<ImageReader url={url} name={file.name}/>;
 else if(kind==='audio'||kind==='video')content=<MediaReader url={url} kind={kind} name={file.name}/>;
 else if(kind==='html')content=<><div className="judex-reader-toolbar">{html&&<Button aria-pressed={mode!=='source'} onPress={()=>setMode('table')}>{w.t('readerRendered')}</Button>}<Button aria-pressed={mode==='source'} onPress={()=>setMode('source')}>{w.t('readerSource')}</Button></div>{html&&mode!=='source'?html:<TextReader url={file.previewData.entries?apiUrl(`/conversations/${w.roomId}/file-versions/${file.id}/entry?path=index.html`):original} name="index.html"/>}</>;
 else if(kind==='archive'&&!entry)content=selected?<><Button onPress={()=>setSelected(undefined)}><ArrowLeft/>{w.t('readerBackArchive')}</Button><ReaderSurface key={selected.path} entry file={{...file,name:selected.path,mime:selected.mime,size:selected.size,previewData:{}}} url={apiUrl(`/conversations/${w.roomId}/file-versions/${file.id}/entry?path=${encodeURIComponent(selected.path)}`)} original={apiUrl(`/conversations/${w.roomId}/file-versions/${file.id}/entry?path=${encodeURIComponent(selected.path)}`)}/></>:<div className="judex-reader-archive" aria-label={w.t('readerArchive')}>{file.previewData.entries?.map(item=><Button key={item.path} onPress={()=>setSelected(item)}><FileText/><span>{item.path}</span><small>{Math.ceil(item.size/1024)} KB</small></Button>)}</div>;
 else content=<Empty title={w.t('readerUnsupported')}><a href={original} download={file.name}>{w.t('vpDownload')}</a></Empty>;
 return <Suspense fallback={<p role="status">{w.t('v2Loading')}</p>}>{content}</Suspense>;
}
