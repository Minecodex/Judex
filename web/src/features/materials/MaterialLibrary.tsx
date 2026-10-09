import {useState} from 'react';
import {useInfiniteQuery} from '@tanstack/react-query';
import {Files, Search, Upload} from 'lucide-react';
import {useWork} from '../work/store';
import {Button} from '../../components/ui/Button';
import {EmptyState, UIDialog} from '../../components/ui/Presentation';
import {UIInput, UIOption, UISelect, UICheckbox} from '../../components/ui/FormControls';
import {request} from '../../lib/api/client';
import {FileCard} from './FileCard';
import {FileThumbnail} from './FileThumbnail';
import {fileSize, materialEvidence, type MaterialItem} from './api';
import {UploadMaterialDialog} from './UploadMaterialDialog';
import type {Evidence} from '../work/types';
import {useMaterialSharing} from './MaterialSharing';
export function useLibrary(query: string, group: string, sort: string, current: boolean) {
  const {project, state, route, mode} = useWork(), sharing=useMaterialSharing(),q = new URLSearchParams({q: query, formatGroup: group, sort, limit: '20'});
  const context=route.conversation?sharing?.context(route.conversation):undefined;
  if(current&&context&&context.kind!=='topic'){
    q.set('linkedObjectType',context.kind);q.set('linkedObjectId',context.id);
  }else if (current&&!context && (route.taskContextId || route.scopeTaskId || route.scopePlanId)) {
    q.set('linkedObjectType', route.taskContextId || route.scopeTaskId ? 'task' : 'plan');
    q.set('linkedObjectId', route.taskContextId || route.scopeTaskId || route.scopePlanId!);
  }
  return useInfiniteQuery({queryKey: ['materials', project.id, state.currentUserId, 'library', q.toString()], initialPageParam: '', enabled: mode === 'api',
    queryFn: ({pageParam}) => request<{items: MaterialItem[]; nextCursor: string | null; totalCount: number}>('/projects/' + project.id + '/materials?' + q + (pageParam ? '&cursor=' + encodeURIComponent(pageParam) : '')),
    getNextPageParam: last => last.nextCursor || undefined});
}
export function MaterialLibrary() {
  const {t, route} = useWork(), [query, setQuery] = useState(''), [group, setGroup] = useState(''), [sort, setSort] = useState('type');
  const [current, setCurrent] = useState(route.page === 'chat'), [adding, setAdding] = useState(false);
  const library = useLibrary(query, group, sort, current), files = library.data?.pages.flatMap(p => p.items) ?? [];
  const total = library.data?.pages[0]?.totalCount ?? files.length;
  return <div className={'judex-material-library' + (route.page === 'chat' ? ' judex-material-library-panel' : '')}>
    <div className="judex-material-heading"><div><h2>{t('matLibrary')}</h2><p>{t('matScopeHint')}</p></div><Button variant="primary" onPress={() => setAdding(true)}><Upload/>{t('matUpload')}</Button></div>
    {route.page === 'chat' && <div className="judex-material-filters"><Button size="sm" aria-pressed={current} onPress={() => setCurrent(true)}>{t('matCurrent')}</Button><Button size="sm" aria-pressed={!current} onPress={() => setCurrent(false)}>{t('matWholeProject')}</Button></div>}
    <div className="judex-material-toolbar"><div className="judex-material-filters">{([['', 'matAll'], ['documents', 'matDocuments'], ['sheets', 'matSheets'], ['images', 'matImages'], ['other', 'matOther']] as const).map(([value, key]) => <Button size="sm" key={value} aria-pressed={group === value} onPress={() => setGroup(value)}>{t(key)}</Button>)}</div>
      <div className="judex-material-search"><Search/><UIInput aria-label={t('matSearch')} placeholder={t('matSearch')} value={query} onChange={e => setQuery(e.target.value)}/></div></div>
    <div className="judex-material-list-heading"><span data-testid="material-count">{t('matCount', {count: total})}{files.length < total && <small> · {t('matLoadedCount', {count: files.length})}</small>}</span>
      <UISelect aria-label={t('matSort')} value={sort} onChange={e => setSort(e.target.value)}><UIOption value="type">{t('matTypeSort')}</UIOption><UIOption value="recent">{t('matRecent')}</UIOption></UISelect></div>
    {library.isError ? <Button onPress={() => void library.refetch()}>{t('matReload')}</Button> : files.length ? <div className="judex-material-grid">{files.map(f => <FileCard key={f.versionId} file={f}/>)}</div>
      : !library.isPending && <EmptyState icon={<Files/>} title={t(query || group ? 'matNoMatch' : 'matEmpty')}/>}
    {library.hasNextPage && <Button disabled={library.isFetchingNextPage} onPress={() => void library.fetchNextPage()}>{t('matMore')}</Button>}
    {adding && <UploadMaterialDialog onClose={() => setAdding(false)}/>}
  </div>;
}
export function MaterialPicker({onClose, onSelect}: {onClose: () => void; onSelect: (files: Evidence[]) => void}) {
  const {t} = useWork(), [query, setQuery] = useState(''), [selected, setSelected] = useState<MaterialItem[]>([]);
  const library = useLibrary(query, '', 'recent', false), items = library.data?.pages.flatMap(p => p.items) ?? [];
  return <UIDialog wide title={t('matChoose')} description={t('matChooseHint')} onClose={onClose} footer={<>
    <span>{t('matSelected', {count: selected.length})}</span><Button variant="outline" onPress={onClose}>{t('matCancel')}</Button>
    <Button variant="primary" disabled={!selected.length} onPress={() => {onSelect(selected.map(materialEvidence));onClose();}}>{t('matChooseConfirm')}</Button>
  </>}>
    <UIInput aria-label={t('matSearch')} placeholder={t('matSearch')} value={query} onChange={e => setQuery(e.target.value)}/>
    <div className="judex-material-picker">{items.map(f => <UICheckbox appearance="card" key={f.versionId} checked={selected.some(v => v.versionId === f.versionId)}
      onChange={e => setSelected(e.target.checked ? [...selected, f] : selected.filter(v => v.versionId !== f.versionId))}>
      <div className="judex-material-picker-card"><FileThumbnail file={f}/><strong title={f.title}>{f.title}</strong><small>{f.authorName || t('matUnrecorded')} · {fileSize(f.size)}</small></div>
    </UICheckbox>)}</div>
    {library.isError && <Button onPress={() => void library.refetch()}>{t('matReload')}</Button>}
    {library.hasNextPage && <Button disabled={library.isFetchingNextPage} onPress={() => void library.fetchNextPage()}>{t('matMore')}</Button>}
  </UIDialog>;
}
