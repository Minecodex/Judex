import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {BookOpen,Check,Copy,Download,FileText,MessageCircle,Send,Terminal,WandSparkles} from 'lucide-react';
import {useWorkspace} from './context';
import {Button,Dialog,IconButton,QueryState} from './ui';
import type {Key} from '../../i18n';
import '../../styles/workspace-guide.css';

function Example({code}:{code:string}){
 const w=useWorkspace(),[copied,setCopied]=useState(false);
 return <div className="judex-guide-example"><pre>{code}</pre><IconButton label={w.t(copied?'guideCopied':'guideCopy')} onPress={()=>{void navigator.clipboard.writeText(code).then(()=>setCopied(true)).catch(()=>w.setToast(w.t('guideCopyFailed')));}}>{copied?<Check/>:<Copy/>}</IconButton></div>;
}
function ClientDownloads(){
 const w=useWorkspace(),downloads=useQuery({queryKey:['v2','client-downloads'],queryFn:async()=>{const r=await fetch('/downloads/manifest.json');if(!r.ok)throw new Error('Download manifest unavailable');return await r.json() as {version:string;artifacts:Record<string,{filename:string;architectures:string[]}>};},retry:false});
 return <QueryState query={downloads}><p className="judex-guide-release">Judex {downloads.data?.version}</p><div className="judex-settings-downloads">{Object.entries(downloads.data?.artifacts||{}).map(([platform,file])=><a className="judex-settings-download" href={'/downloads/'+encodeURIComponent(file.filename)} key={platform} download><Download/><span>{platform==='macos'?'macOS':platform==='windows'?'Windows':'Judex Skill'}<small>{file.architectures.length?'x64 / ARM64':downloads.data?.version}</small></span></a>)}</div></QueryState>;
}
export function UserGuide(){
 const w=useWorkspace(),[page,setPage]=useState<Key>('guideStart');
 const entries:{id:Key;icon:typeof Terminal}[]=[{id:'guideStart',icon:Terminal},{id:'guideRead',icon:FileText},{id:'guidePublish',icon:Send},{id:'guideSkill',icon:WandSparkles},{id:'guideControl',icon:MessageCircle},{id:'guideDownloads',icon:Download}];
 const project=w.projectId||'<project-id>',group=w.groupId||'<group-id>',text=w.locale==='en'?'Design is ready for review':'设计稿已完成，请分析';
 return <Dialog wide className="judex-guide-modal" title={w.t('guideTitle')} description={w.t('guideIntro')}><div className="judex-guide-layout"><nav aria-label={w.t('guideTitle')}><span className="judex-guide-mark"><BookOpen/>CLI + Skill</span>{entries.map(({id,icon:Icon})=><Button key={id} aria-current={page===id?'page':undefined} onPress={()=>setPage(id)}><Icon/>{w.t(id)}</Button>)}</nav><article className="judex-guide-content" key={page}><h2>{w.t(page)}</h2>
 {page==='guideStart'&&<><p>{w.t('guideInstall')}</p><Example code={'judex version\njudex status'}/><p>{w.t('guideLogin')}</p><Example code={`judex auth login --server ${location.origin}\njudex project list\njudex project use ${project}\njudex group list\njudex group use ${group}\njudex group show`}/></>}
 {page==='guideRead'&&<><p>{w.t('guideReadHint')}</p><Example code={'judex group show\njudex message list\njudex context read\njudex file list\njudex file download <version-id> ./notes.md'}/><Example code={'judex message list --before <message-sequence>\njudex context read --kind message --ref <message-id> --offset 0 --limit 8000\njudex watch --after <event-sequence>'}/></>}
 {page==='guidePublish'&&<><p>{w.t('guidePublishHint')}</p><Example code={`judex push --text "${text}" --file ./notes.md\njudex push --record-only --text "${text}" --file ./notes.md\njudex message send --text "${text}"\njudex message send --text "${text}" --mention-ai <role-agent-id>`}/><p>{w.t('guideVersionHint')}</p><Example code={'judex file upload ./notes.md\njudex file list\njudex file upload ./notes.md --update <file-id> --expected-version <version>'}/></>}
 {page==='guideSkill'&&<><p>{w.t('guideSkillHint')}</p><Example code={'judex skill install --target codex\njudex skill install --target claude-code\njudex skill install --target path --path ./my-skills/judex'}/><Example code={w.t('guideSkillExample')}/><p className="judex-guide-note">{w.t('guideSkillBoundary')}</p></>}
 {page==='guideControl'&&<><p>{w.t('guideControlHint')}</p><Example code={`judex message send --text "${text}" --confirmation "${w.locale==='en'?'Design review':'设计确认'}" --mention-human <user-id>\njudex message send --text "${w.locale==='en'?'Agreed':'同意'}" --reply-to <message-id> --decision agree\njudex run list\njudex inbox`}/><p>{w.t('guideRecoveryHint')}</p><Example code={'judex pending\njudex watch --after <event-sequence>'}/></>}
 {page==='guideDownloads'&&<><p>{w.t('guideDownloadHint')}</p><ClientDownloads/></>}
 </article></div></Dialog>;
}
