import {useState} from 'react';
import {Card} from '@heroui/react';
import {FileText, Info, Trash2} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {PersonAvatar} from '../../components/ui/Presentation';
import {useWork} from '../work/store';
import {fileSize, materialDate, useMaterialPreview, useMaterialVersion, type MaterialItem} from './api';
import {MaterialDialog, DeleteMaterialDialog} from './MaterialDialog';
import {FileThumbnail} from './FileThumbnail';
import {MaterialStatus} from './MaterialStatus';
export function FileCard({file, compact = false}: {file: MaterialItem; compact?: boolean}) {
  const {t, locale} = useWork(), [dialog, setDialog] = useState<'preview' | 'details' | 'delete'>();
  const preview = useMaterialPreview(file.versionId, !file.deletedAt);
  const modal = dialog === 'delete' ? <DeleteMaterialDialog file={file} onClose={() => setDialog(undefined)}/>
    : dialog && <MaterialDialog file={file} initial={dialog} onClose={() => setDialog(undefined)}/>;
  if (file.deletedAt) return <><Card className="judex-material-card judex-material-deleted" data-material-id={file.id} data-version-id={file.versionId}>
    <FileText/><div><strong title={file.title}>{file.title}</strong><span>{t('matDeleted')} · {t('matHistoryRetained')}</span></div>
    <Button size="sm" onPress={() => setDialog('details')}><Info/>{t('matDetails')}</Button>
  </Card>{modal}</>;
  return <><Card className={'judex-material-card' + (compact ? ' judex-material-compact' : '')} data-material-id={file.id} data-version-id={file.versionId}>
    <Button className="judex-material-cover" aria-label={t('matPreview') + ' ' + file.title} onPress={() => setDialog('preview')}><FileThumbnail file={file}/></Button>
    <Card.Content className="judex-material-card-content">
      <Button className="judex-material-name" title={file.title} onPress={() => setDialog('preview')}>{file.title}</Button>
      <div className="judex-material-meta"><span>{fileSize(file.size)} · v{file.revision}</span><span className="judex-material-tag">{t(file.associations.some(a => a.type === 'task') ? 'matTask' : file.associations.length ? 'matPlan' : 'matShared')}</span></div>
      <div className="judex-material-person"><PersonAvatar name={file.authorName || t('matUnrecorded')} small/><div><span>{file.authorName || t('matUnrecorded')}</span><time>{materialDate(file.uploadedAt, locale, t('matUnrecorded'))}</time></div></div>
      <MaterialStatus file={file} preview={preview.data}/>
    </Card.Content>
    <Card.Footer className="judex-material-card-actions"><Button size="sm" onPress={() => setDialog('details')}><Info/>{t('matDetails')}</Button>{file.canDelete && <Button size="sm" className="judex-material-delete" onPress={() => setDialog('delete')}><Trash2/>{t('matDelete')}</Button>}</Card.Footer>
  </Card>{modal}</>;
}
function FileReference({versionId, name}: {versionId: string; name: string}) {
  const {t} = useWork(), data = useMaterialVersion(versionId);
  return data.data ? <FileCard file={data.data} compact/> : <div className="judex-material-reference-state"><FileText/><span>{name}</span>{data.isError && <Button size="sm" onPress={() => void data.refetch()}>{t('matReload')}</Button>}</div>;
}
export function MaterialReferences({materials}: {materials: {versionId: string; name: string}[]}) {
  return <div className="judex-material-message-grid">{materials.map(m => <FileReference key={m.versionId} {...m}/>)}</div>;
}
