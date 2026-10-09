import {useWork} from '../work/store';
import {usePreviewReading,type MaterialItem,type PreviewInfo} from './api';
export function MaterialStatus({file, preview}: {file: MaterialItem; preview?: PreviewInfo}) {
  const {t} = useWork(), reading=usePreviewReading(file.versionId),status = preview?.status ?? file.previewStatus;
  return <div className="judex-material-status" data-original-status={file.originalStatus} data-preview-status={reading.failed?'read_failed':status}>
    <span>{t(file.originalStatus === 'ready' ? 'matReady' : 'matOriginalUnavailable')}</span>
    <span className={status === 'failed'||reading.failed ? 'judex-material-status-failed' : ''}>{t(reading.failed?'matReadError':status === 'ready' ? 'matPreviewReady' : status === 'failed' ? 'matFailed' : status === 'unsupported' ? 'matNoPreview' : 'matPending')}</span>
  </div>;
}
