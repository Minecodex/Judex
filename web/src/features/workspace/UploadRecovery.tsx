import {useWorkspace} from './context';
import {Button} from './ui';

export function UploadRecovery() {
  const w = useWorkspace(), batch = w.uploadBatch;
  if (!batch || batch.running) return null;
  const done = batch.items.filter(item => item.confirmed).length;
  return <section className="judex-v2-upload-recovery" aria-label={w.t('v2UploadBatch', {done, total: batch.items.length})} data-testid="upload-recovery">
    <strong>{w.t('v2UploadBatch', {done, total: batch.items.length})}</strong>
    <ul>{batch.items.map((item, index) => <li key={index}><span>{item.name}</span><small>{w.t(item.confirmed ? 'v2UploadConfirmed' : 'v2UploadUnconfirmed')}</small></li>)}</ul>
    <p>{w.t('v2UploadRetained')}</p>
    <div><Button variant="primary" onPress={() => void w.resumeUpload()}>{w.t('v2UploadResume')}</Button><Button onPress={w.closeUpload}>{w.t('v2UploadClose')}</Button></div>
  </section>;
}
