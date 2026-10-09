import {useEffect, useState} from 'react';
import {useInfiniteQuery, useQueryClient} from '@tanstack/react-query';
import {Compass, Download, MessageCircle, Paperclip, RefreshCw, Upload} from 'lucide-react';
import {UIDialog, PersonAvatar} from '../../components/ui/Presentation';
import {Button} from '../../components/ui/Button';
import {useWork} from '../work/store';
import {useSurfaceNavigation} from '../work/SurfaceNavigation';
import {request} from '../../lib/api/client';
import {useWorkObjectNavigation} from '../cooperation/objectNavigation';
import {fileSize, formatLabel, materialDate, materialURL, originalURL, useMaterialPreview, usePreviewContents, usePreviewReading, versionPath, type MaterialItem, type MaterialUsage} from './api';
import {PdfReader} from './PdfReader';
import {MarkdownReader} from './MarkdownReader';
import {MaterialStatus} from './MaterialStatus';
import {useMaterialSharing} from './MaterialSharing';
import type {Key} from '../../i18n';
import {materialErrorKey} from './errors';

function PreviewContents({file}: {file: MaterialItem}) {
  const {project, t} = useWork(), preview = useMaterialPreview(file.versionId);
  const contents = usePreviewContents(file, preview.data);
  const reading=usePreviewReading(file.versionId);
  const [error, setError] = useState(''), [zoom, setZoom] = useState(100), [htmlURL, setHtmlURL] = useState<string | null>(null),[htmlAttempt,setHtmlAttempt]=useState(0);
  useEffect(() => {
    let alive = true;
    if (file.kind === 'html_bundle') {
      setHtmlURL(null);
      void request<{previewUrl: string | null}>('/projects/' + project.id + '/materials/' + file.id + '/versions/' + file.versionId + '/preview-session',
        {method: 'POST', idempotencyKey: crypto.randomUUID()}).then(data => {if (alive) setHtmlURL(data.previewUrl || '');}).catch(() => {if (alive) setError(t('matReadError'));});
    }
    return () => {alive = false;};
  }, [file.versionId,htmlAttempt]);
  useEffect(() => {if (file.kind !== 'html_bundle' && preview.data?.status === 'not_requested') void preview.prepare().catch(() => setError(t('matReadError')));}, [preview.data?.status]);
  const retry = (force = false) => {setError('');reading.clear();if(file.kind==='html_bundle')setHtmlAttempt(value=>value+1);else void preview.prepare(force).catch(() => setError(t('matReadError')));};
  useEffect(()=>{if(preview.isError||contents.isError)reading.fail();},[preview.isError,contents.isError]);
  const fail=()=>{reading.fail();setError(t('matReadError'));};
  if (preview.isError || contents.isError || error||reading.failed) return <div className="judex-material-preview-state" role="alert"><p>{error || t('matReadError')}</p><Button onPress={() => retry(true)}>{t('matRetry')}</Button></div>;
  if (file.kind === 'html_bundle') return htmlURL === null ? <div className="judex-material-preview-state">{t('matPending')}</div> : htmlURL ? <iframe className="judex-material-html" title={file.title} src={htmlURL} sandbox="allow-scripts"/> : <div className="judex-material-preview-state">{t('matUnsupported')}</div>;
  if (!preview.data || ['not_requested', 'pending'].includes(preview.data.status)) return <div className="judex-material-preview-state">{t('matPending')}</div>;
  if (preview.data.status === 'unsupported') return <div className="judex-material-preview-state">{t('matUnsupported')}</div>;
  if (preview.data.status === 'failed') {
    const reason = preview.data.error ?? '';
    const key: Key = /limit/.test(reason) ? 'matFailureLimit' : /unsafe/.test(reason) ? 'matFailureUnsafe' : /unavailable/.test(reason) ? 'matFailureUnavailable' : /Office/.test(reason) ? 'matFailureOffice' : 'matFailureInvalid';
    return <div className="judex-material-preview-state"><strong>{t('matFailed')}</strong><p>{t(key)}</p><Button variant="outline" onPress={() => retry()}><RefreshCw/>{t('matRetry')}</Button></div>;
  }
  const url = materialURL(preview.data.contentUrl!);
  if (preview.data.kind === 'pdf') return <PdfReader url={url} onError={fail}/>;
  if (preview.data.kind === 'image') return <div className="judex-material-image"><div className="judex-material-reader-toolbar">
    <Button size="sm" aria-label={t('matZoomOut')} onPress={() => setZoom(Math.max(50, zoom - 25))}>−</Button><span>{zoom}%</span><Button size="sm" aria-label={t('matZoomIn')} onPress={() => setZoom(Math.min(200, zoom + 25))}>＋</Button>
  </div><img src={url} alt={file.title} style={{width: zoom + '%'}} onError={fail}/></div>;
  if (contents.isPending) return <div className="judex-material-preview-state">{t('matPending')}</div>;
  if (preview.data.kind === 'archive') return <div className="judex-material-archive">{contents.data?.entries.map((v, i) => <div key={i}><Paperclip/><span>{v.name}</span><small>{fileSize(v.size)}</small></div>)}</div>;
  return file.format === 'markdown' ? <MarkdownReader text={contents.data?.text ?? ''}/> : <pre className="judex-material-source">{contents.data?.text}</pre>;
}
export function MaterialDialog({file, initial, onClose}: {file: MaterialItem; initial: 'preview' | 'details'; onClose: () => void}) {
  const p = useWork(), {go} = useSurfaceNavigation(), openObject = useWorkObjectNavigation(), sharing = useMaterialSharing();
  const [tab, setTab] = useState(initial), preview = useMaterialPreview(file.versionId);
  const uses = useInfiniteQuery({queryKey: ['materials', p.project.id, p.state.currentUserId, 'usage', file.versionId], initialPageParam: '', enabled: tab === 'details',
    queryFn: ({pageParam}) => request<{items: MaterialUsage[]; nextCursor: string | null}>(versionPath(p.project.id, file.versionId) + '/usages?limit=10' + (pageParam ? '&cursor=' + encodeURIComponent(pageParam) : '')),
    getNextPageParam: last => last.nextCursor || undefined});
  const source = (s: string) => p.t(s === 'web' ? 'matUploadSourceWeb' : s === 'cli' ? 'matUploadSourceCli' : s === 'agent' ? 'matUploadSourceAgent' : 'matUnrecorded');
  const missing = p.t('matUnrecorded'), date = (value: string | null) => materialDate(value, p.locale, missing);
  const metadata = [[p.t('matUploader'), file.authorName || missing], [p.t('matUploaded'), date(file.uploadedAt)], [p.t('matSource'), source(file.source)], [p.t('matVersion'), 'v' + file.revision], [p.t('matSize'), fileSize(file.size)], [p.t('matFormat'), formatLabel(file)]];
  const history = uses.data?.pages.flatMap(page => page.items) ?? [];
  return <UIDialog title={file.title} description={formatLabel(file) + ' · ' + fileSize(file.size) + ' · v' + file.revision} wide onClose={onClose} footer={<>
    <a className="judex-material-download" href={originalURL(p.project.id, file.versionId)} download={file.title}><Download/>{p.t('matDownload')}</a>
    {sharing && !file.deletedAt && <Button variant="primary" onPress={() => {onClose();sharing.open(file);}}><MessageCircle/>{p.t('matSend')}</Button>}
    <Button variant="outline" onPress={onClose}>{p.t('matClose')}</Button>
  </>}><div className="judex-material-dialog">
    <div className="judex-material-dialog-tabs"><Button aria-pressed={tab === 'preview'} onPress={() => setTab('preview')}>{p.t('matFileContents')}</Button><Button aria-pressed={tab === 'details'} onPress={() => setTab('details')}><Compass/>{p.t('matOrigin')}</Button>{file.deletedAt && <span>{p.t('matDeleted')}</span>}</div>
    <MaterialStatus file={file} preview={preview.data}/>
    {tab === 'preview' ? <div className="judex-material-reading"><div className="judex-material-viewer"><PreviewContents file={file}/></div>
      <aside><h3>{p.t('matPurpose')}</h3><p>{file.purpose || p.t('matNoPurpose')}</p><PersonAvatar name={file.authorName || missing}/><span>{file.authorName || missing}</span><small>{date(file.uploadedAt)}</small>
        {file.associations.map(a => <div key={a.type + a.id}><small>{p.t(a.type === 'task' ? 'matTask' : 'matPlan')}</small><strong>{a.name}</strong></div>)}
        <Button variant="outline" size="sm" onPress={() => setTab('details')}>{p.t('matUses')}</Button>
      </aside></div> : <>
      <section className="judex-material-purpose"><h2><Compass/>{p.t('matPurpose')}</h2><p>{file.purpose || p.t('matNoPurpose')}</p></section>
      <div className="judex-material-details-columns"><section><h3>{p.t('matInfo')}</h3><dl>{metadata.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
        <div className="judex-material-context"><small>{p.t('matProject')}</small><strong>{p.text(p.project.title)}</strong>
          {file.associations.map(a => <div key={a.type + a.id}><small>{p.t(a.type === 'task' ? 'matTask' : 'matPlan')}</small><Button size="sm" variant="outline" onPress={() => {onClose();openObject(a.type === 'task' ? 'task' : 'plan', a.id);}}>{a.name}</Button></div>)}
        </div></section>
        <section><h3>{p.t('matUses')}</h3><div className="judex-material-timeline"><article><Upload/><div><strong>{p.t('matUploadRecord')}</strong><p>{file.authorName || missing} · {source(file.source)}</p><time>{date(file.uploadedAt)}</time></div></article>
          {history.map(u => <article key={u.id}><Paperclip/><div><strong>{p.t(({progress: 'matRecordProgress', delivery: 'matRecordDelivery', message: 'matRecordMessage', material: 'matRecordMaterial', reference: 'matRecordReference'} as Record<string, Key>)[u.kind] ?? 'matRegistered')}</strong>
            {u.planId && <span>{p.t('matPlan')}: {u.planName || missing}</span>}{u.taskId && <span>{p.t('matTask')}: {u.taskName || missing}</span>}{u.topicId && <span>{u.topicName || missing}</span>}
            <p>{u.description || missing}</p><small>{u.actorName || missing} · {source(u.source)}</small><time>{date(u.createdAt)}</time>
            {u.topicId && <Button size="sm" variant="ghost" onPress={() => {onClose();go({view: 'topic', id: u.topicId!, scopeTaskId: u.taskId ?? undefined, scopePlanId: u.taskId ? undefined : u.planId ?? undefined, taskContextId: u.taskId ?? undefined});}}>{p.t('matOpenDiscussion')}</Button>}
          </div></article>)}</div>
          {!uses.isPending && !uses.isError && !history.length && <p>{p.t('matNoUses')}</p>}
          {uses.isError && <Button onPress={() => void uses.refetch()}>{p.t('matReload')}</Button>}
          {uses.hasNextPage && <Button size="sm" disabled={uses.isFetchingNextPage} onPress={() => void uses.fetchNextPage()}>{p.t('matMore')}</Button>}
        </section></div>
    </>}
  </div></UIDialog>;
}
export function DeleteMaterialDialog({file, onClose}: {file: MaterialItem; onClose: () => void}) {
  const {t, project} = useWork(), client = useQueryClient(), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const remove = async () => {
    if(busy)return;setBusy(true);setError('');
    try {await request('/projects/' + project.id + '/materials/' + file.id, {method: 'DELETE', idempotencyKey: crypto.randomUUID(), body: JSON.stringify({expectedCurrentVersionId: file.currentVersionId})});await client.invalidateQueries({queryKey: ['materials', project.id]});onClose();}
    catch (e) {setError(t(materialErrorKey(e)));setBusy(false);}
  };
  return <UIDialog title={t('matDeleteTitle')} onClose={onClose} dismissable={!busy} footer={<>
    <Button variant="outline" onPress={onClose} disabled={busy}>{t('matCancel')}</Button><Button variant="danger" onPress={remove} isPending={busy}>{t('matConfirmDelete')}</Button>
  </>}><strong>{file.title}</strong><p>{t('matDeleteHint')}</p>{error && <p role="alert">{error}</p>}</UIDialog>;
}
