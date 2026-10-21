import {useEffect,useId,useMemo,useState,type ReactNode} from 'react';
import {Input} from '@heroui/react';
import {useQuery} from '@tanstack/react-query';
import {Copy,Search,WrapText} from 'lucide-react';
import {useWorkspace} from '../context';
import {Button,Choice,Empty,IconButton} from '../ui';
import {MarkdownReader} from '../../../components/readers/MarkdownReader';
import {renderDiagram} from '../../../components/readers/diagram';
import {extension,languages} from './formats';
import {loadBytes,TEXT_LIMIT} from './load';
export default function TextReader({url,name,kind='text'}:{url:string;name:string;kind?:string}){
 const w=useWorkspace(),[encoding,setEncoding]=useState('utf-8'),[mode,setMode]=useState(kind==='markdown'||kind==='diagram'?'rendered':'source'),[search,setSearch]=useState(''),[wrap,setWrap]=useState(false),[page,setPage]=useState(0),[code,setCode]=useState<string[]>([]),[tree,setTree]=useState<unknown>(),[parseError,setParseError]=useState(false),[svg,setSVG]=useState('');const id=useId().replace(/[^a-zA-Z0-9]/g,'');
 const query=useQuery({queryKey:['v2',w.user.id,'reader-text',url],queryFn:({signal})=>loadBytes(url,TEXT_LIMIT,signal),retry:false});
 const text=useMemo(()=>query.data?new TextDecoder(encoding).decode(query.data.bytes):'',[query.data,encoding]);const ext=extension(name),structured=['json','jsonl','yaml','yml','xml','toml'].includes(ext);
 useEffect(()=>{setPage(0);setParseError(false);setCode([]);setTree(undefined);let live=true;void(async()=>{
  const {default:hljs}=await import('highlight.js/lib/common');const language=languages[ext];if(live)setCode(language&&hljs.getLanguage(language)?text.slice(0,500000).split('\n').map(line=>hljs.highlight(line,{language,ignoreIllegals:true}).value):[]);
  if(structured){let value:unknown;if(ext==='json')value=JSON.parse(text);else if(ext==='jsonl')value=text.split(/\r?\n/).filter(Boolean).map(l=>JSON.parse(l));else if(ext==='yaml'||ext==='yml')value=(await import('yaml')).parse(text,{maxAliasCount:100});else if(ext==='toml')value=(await import('smol-toml')).parse(text);else{const doc=new DOMParser().parseFromString(text,'application/xml');if(doc.querySelector('parsererror'))throw Error();value=xmlNode(doc.documentElement,0);}if(live)setTree(value);}
 })().catch(()=>{if(live)setParseError(true);});return()=>{live=false;};},[text,ext,structured]);
 useEffect(()=>{if(kind!=='diagram'||mode!=='rendered'||!text)return;let live=true;setSVG('');setParseError(false);void renderDiagram('judexfile'+id,text).then(out=>{if(live)setSVG(out.svg);}).catch(()=>{if(live)setParseError(true);});return()=>{live=false;};},[text,kind,mode,w.theme,id]);
 const lines=text.split('\n'),filtered=useMemo(()=>lines.map((line,index)=>({line,index})).filter(v=>!search||v.line.toLocaleLowerCase().includes(search.toLocaleLowerCase())),[text,search]);const pages=Math.max(1,Math.ceil(filtered.length/300)),current=Math.min(page,pages-1);
 if(query.isPending)return <p role="status">{w.t('v2Loading')}</p>;if(query.isError)return <Empty title={w.t('readerFailed')}><Button onPress={()=>void query.refetch()}>{w.t('v2Retry')}</Button></Empty>;
 return <section className="judex-reader-text" data-testid="text-reader"><div className="judex-reader-toolbar">
 {(kind==='markdown'||kind==='diagram'||structured)&&<><Button aria-pressed={mode==='source'} onPress={()=>setMode('source')}>{w.t('readerSource')}</Button><Button aria-pressed={mode==='rendered'} onPress={()=>setMode('rendered')}>{w.t(structured?'readerStructure':'readerRendered')}</Button></>}
 <Choice label={w.t('readerTextEncoding')} value={encoding} onChange={setEncoding} items={[{id:'utf-8',name:'UTF-8'},{id:'gb18030',name:'GB18030 / GBK'},{id:'utf-16le',name:'UTF-16 LE'}]}/><IconButton label={w.t('readerWrap')} aria-pressed={wrap} onPress={()=>setWrap(!wrap)}><WrapText/></IconButton><IconButton label={w.t('readerCopy')} onPress={()=>void navigator.clipboard.writeText(text).then(()=>w.setToast(w.t('readerCopied'))).catch(()=>w.setToast(w.t('v2Failed')))}><Copy/></IconButton>
 </div>{query.data.partial&&<p className="judex-reader-notice">{w.t('readerPartial')}</p>}
 {mode==='rendered'?parseError?<p role="alert">{w.t('readerParseWarning')}</p>:kind==='markdown'?<MarkdownReader text={text}/>:kind==='diagram'?svg?<div className="judex-reader-diagram" dangerouslySetInnerHTML={{__html:svg}}/>:<p>{w.t('v2Loading')}</p>:<div className="judex-reader-tree">{treeNode(tree,'root',0,{left:1500})}</div>:<>
 <div className="judex-reader-search"><Search/><Input aria-label={w.t('readerSearch')} placeholder={w.t('readerSearch')} value={search} onChange={e=>{setSearch(e.target.value);setPage(0);}}/></div>
 <div className={'judex-reader-code'+(wrap?' judex-reader-wrap':'')}><table><tbody>{filtered.slice(current*300,(current+1)*300).map(v=><tr key={v.index}><th scope="row">{v.index+1}</th><td>{code[v.index]!==undefined?<code dangerouslySetInnerHTML={{__html:code[v.index]}}/>:<code>{v.line}</code>}</td></tr>)}</tbody></table></div>
 <div className="judex-reader-toolbar"><small>{w.t('readerRows',{n:filtered.length})}</small><span/><Button isDisabled={!current} onPress={()=>setPage(current-1)}>{w.t('readerPrevious')}</Button><small>{w.t('readerPage',{page:current+1,pages})}</small><Button isDisabled={current===pages-1} onPress={()=>setPage(current+1)}>{w.t('readerNext')}</Button></div>
 </>}
 </section>;
}
function treeNode(value:unknown,name:string,depth:number,budget:{left:number}):ReactNode{
 if(--budget.left<0||depth>20)return <span key={name}>…</span>;
 if(value&&typeof value==='object'){const entries=Object.entries(value);return <details key={name} open={depth===0}><summary>{name} <small>{Array.isArray(value)?'[':'{'}{entries.length}{Array.isArray(value)?']':'}'}</small></summary><div>{entries.slice(0,300).map(([key,v])=>treeNode(v,key,depth+1,budget))}{entries.length>300&&<p>…</p>}</div></details>;}
 return <p key={name}><strong>{name}: </strong><span>{String(value)}</span></p>;
}
function xmlNode(node:Element,depth:number):unknown{if(depth>20)return '…';return {tag:node.tagName,attributes:Object.fromEntries([...node.attributes].map(a=>[a.name,a.value])),...(node.children.length?{children:[...node.children].slice(0,300).map(e=>xmlNode(e,depth+1))}:{text:node.textContent})};}
