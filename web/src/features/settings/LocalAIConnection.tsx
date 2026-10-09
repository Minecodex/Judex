import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Copy, Download, Laptop, Monitor, Puzzle, Terminal } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { PageHeading } from '../../components/ui/Presentation';
import { UICard, UIWarning } from '../../components/ui/FormControls';
import { clientDownloads, downloadArtifact } from '../../lib/clientDownloads';
import { useWork } from '../work/store';

export function LocalAIConnection() {
  const {t,go}=useWork();const [notice,setNotice]=useState('');
  const query=useQuery({queryKey:['clientDownloads'],queryFn:clientDownloads,retry:false,staleTime:60000});
  const artifacts=query.data?.artifacts;
  const copy=async(text:string)=>{try{await navigator.clipboard.writeText(text);setNotice(t('connectionCopied'));}catch{setNotice(t('connectionCopyFailed'));}};
  const server=location.origin;
  const platforms=[{key:'windows',label:t('connectionWindowsCommand'),executable:'.\\judex.cmd'},{key:'macos',label:t('connectionMacCommand'),executable:'./judex'}];
  const commands=platforms.map(platform=>({...platform,text:`${platform.executable} --server ${server} auth login`}));
  const installCommands=platforms.map(platform=>({...platform,text:`${platform.executable} skill install --target codex\n${platform.executable} skill install --target claude-code`}));
  const size=(bytes:number)=>bytes<1024*1024?`${Math.round(bytes/1024)} KB`:`${(bytes/1024/1024).toFixed(1)} MB`;
  return <section className="judex-local-connection" data-testid="local-ai-connection">
    <PageHeading title={t('connectionTitle')} description={t('connectionHint')}/>
    {query.isPending&&<p role="status" className="judex-connection-status">{t('connectionLoading')}</p>}
    {query.isError&&<UIWarning role="alert">{t('connectionUnavailable')}<Button size="sm" onPress={()=>void query.refetch()}>{t('connectionRetry')}</Button></UIWarning>}
    <div className="judex-connection-downloads">
      <UICard className="judex-download-card"><div className="judex-download-card-heading"><span className="judex-download-mark"><Terminal/></span><div><h2>{t('connectionCLI')}</h2><p>{t('connectionCLIHint')}</p></div></div>
        <div className="judex-platform-downloads">
          <div><Button variant="primary" disabled={!artifacts} data-testid="download-cli-windows" onPress={()=>artifacts&&downloadArtifact(artifacts.windows)}><Monitor/>{t('connectionWindows')}<Download/></Button><small>{t('connectionWindowsArch')}{artifacts&&' · '+size(artifacts.windows.size)}</small></div>
          <div><Button variant="outline" disabled={!artifacts} data-testid="download-cli-macos" onPress={()=>artifacts&&downloadArtifact(artifacts.macos)}><Laptop/>{t('connectionMac')}<Download/></Button><small>{t('connectionMacArch')}{artifacts&&' · '+size(artifacts.macos.size)}</small></div>
        </div>
      </UICard>
      <UICard className="judex-download-card"><div className="judex-download-card-heading"><span className="judex-download-mark judex-color-violet"><Puzzle/></span><div><h2>{t('connectionSkills')}</h2><p>{t('connectionSkillsHint')}</p></div></div>
        <Button variant="outline" disabled={!artifacts} data-testid="download-skills" onPress={()=>artifacts&&downloadArtifact(artifacts.skills)}><Download/>{t('connectionDownloadSkills')}</Button>
        {query.data&&<small className="judex-download-meta">{t('connectionVersion',{version:query.data.version})} · {size(query.data.artifacts.skills.size)}</small>}
      </UICard>
    </div>
    <UICard className="judex-connection-guide"><h2>{t('connectionStart')}</h2><ol><li>{t('connectionStepOne')}</li><li>{t('connectionStepTwo')}</li><li>{t('connectionStepThree')}</li></ol>
      <div className="judex-connection-commands">{commands.map(command=><div key={command.key}><div className="judex-command-heading"><span>{command.label}</span><Button size="sm" isIconOnly aria-label={t('connectionCopy')+' · '+command.label} onPress={()=>void copy(command.text)}><Copy/></Button></div><code>{command.text}</code></div>)}</div>
      <p className="judex-connection-install-title">{t('connectionInstall')}</p><div className="judex-connection-commands">{installCommands.map(command=><div key={command.key}><div className="judex-command-heading"><span>{command.label}</span></div><code>{command.text}</code></div>)}</div>
      {notice&&<p role="status" className="judex-connection-status">{notice}</p>}
    </UICard>
    <div className="judex-connection-footer"><p>{t('connectionBoundary')}</p><Button variant="ghost" onPress={()=>go({settingsSection:undefined,view:'tasks',id:undefined})}>{t('connectionTasks')}</Button></div>
  </section>;
}
