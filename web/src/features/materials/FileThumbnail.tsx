import {useEffect} from 'react';
import {Archive, Braces, FileImage, FileSpreadsheet, FileText, Presentation} from 'lucide-react';
import {useWork} from '../work/store';
import {formatLabel, materialURL, useMaterialPreview, usePreviewContents, usePreviewReading, type MaterialItem} from './api';
import {PdfReader} from './PdfReader';
export function FileThumbnail({file}: {file: MaterialItem}) {
  const {t} = useWork(), preview = useMaterialPreview(file.versionId, !file.deletedAt);
  const contents = usePreviewContents(file, preview.data, true);
  const reading=usePreviewReading(file.versionId);
  useEffect(() => {
    if (preview.data?.status === 'not_requested') void preview.prepare().catch(() => {});
  }, [preview.data?.status]);
  const Icon = file.format === 'xlsx' ? FileSpreadsheet : file.format === 'image' ? FileImage : file.format === 'pptx' ? Presentation : file.format === 'json' ? Braces : file.format === 'archive' ? Archive : FileText;
  const thumb = preview.data?.thumbnail;
  return <div className={'judex-material-thumbnail judex-material-format-' + file.format} aria-hidden="true">
    <span className="judex-material-format-badge">{formatLabel(file)}</span>
    {thumb?.kind === 'pdf' ? <PdfReader thumbnail url={materialURL(thumb.url)} onError={reading.fail}/>
      : thumb?.kind === 'image' ? <img alt="" src={materialURL(thumb.url)} loading="lazy" onError={reading.fail}/>
      : thumb?.kind === 'text' && contents.data ? <pre className="judex-material-code-art">{contents.data.text.slice(0, 1100)}</pre>
      : thumb?.kind === 'archive' && contents.data ? <div className="judex-material-folder-art"><Archive/><strong>{file.title}</strong><span>{t('matArchiveCount', {count: contents.data.entries.filter(e => !e.directory).length})}</span><div>{contents.data.entries.slice(0, 3).map(e => <small key={e.name}>{e.name}</small>)}</div></div>
      : <div className="judex-material-file-art"><Icon/><strong>{file.title}</strong><span>{t(preview.data?.status === 'pending' ? 'matPending' : preview.data?.status === 'failed' ? 'matFailed' : preview.data?.status === 'unsupported' ? 'matNoPreview' : 'matPreview')}</span></div>}
  </div>;
}
