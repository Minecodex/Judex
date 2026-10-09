import {useState} from 'react';
import {useQueryClient} from '@tanstack/react-query';
import {useWork} from '../work/store';
import {request} from '../../lib/api/client';
import {UIDialog} from '../../components/ui/Presentation';
import {Button} from '../../components/ui/Button';
import {FormField, UIFilePicker, UITextArea} from '../../components/ui/FormControls';
import {uploadFile} from '../work/apiActions';
import {MaterialWorkContext} from './MaterialWorkContext';
import {useMaterialSharing} from './MaterialSharing';
import {materialErrorKey} from './errors';
export function UploadMaterialDialog({onClose}: {onClose: () => void}) {
  const {project, t, route} = useWork(), client = useQueryClient();
  const sharing=useMaterialSharing(),context=route.conversation?sharing?.context(route.conversation):undefined;
  const [files, setFiles] = useState<File[]>([]), [purpose, setPurpose] = useState('');
  const [plan, setPlan] = useState(context?context.kind==='plan'?context.id:'':route.scopePlanId ?? ''), [task, setTask] = useState(context?context.kind==='task'?context.id:'':route.taskContextId ?? route.scopeTaskId ?? '');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [versions, setVersions] = useState<string[]>([]);
  const [submissionId] = useState(() => crypto.randomUUID());
  const save = async () => {
    if (!files.length || !purpose.trim() || busy) return;
    setBusy(true);setError('');
    try {
      const ids = [...versions];
      for (let i = ids.length; i < files.length; i++) {ids.push(await uploadFile(project.id, files[i], purpose));setVersions([...ids]);}
      await request('/projects/' + project.id + '/submissions', {method: 'POST', idempotencyKey: submissionId,
        body: JSON.stringify({clientSubmissionId: submissionId, purpose: 'material', text: purpose,
          ...(task ? {taskId: task} : plan ? {planId: plan} : {}), materialVersionIds: ids})});
      await client.invalidateQueries({queryKey: ['materials', project.id]});onClose();
    } catch (e) {setError(t(materialErrorKey(e)));setBusy(false);}
  };
  return <UIDialog title={t('matUpload')} onClose={onClose} dismissable={!busy} footer={<>
    <Button variant="outline" disabled={busy} onPress={onClose}>{t('matCancel')}</Button>
    <Button variant="primary" isPending={busy} disabled={!files.length || !purpose.trim()} onPress={save}>{t('matRegister')}</Button>
  </>}>
    <UIFilePicker multiple aria-label={t('matSelectFile')} disabled={busy || !!versions.length} onChange={e => {setFiles(Array.from(e.target.files ?? []));setVersions([]);}}>{t('matSelectFile')}</UIFilePicker>
    <ul className="judex-material-selected-files">{files.map((f,index) => <li key={index+':'+f.name}><span title={f.name}>{f.name}</span>{index<versions.length&&<small>{t('matUploadRecord')}</small>}</li>)}</ul>
    <FormField label={t('matPurposeInput')}><UITextArea placeholder={t('matPurposePlaceholder')} value={purpose} disabled={busy || !!versions.length} onChange={e => setPurpose(e.target.value)}/></FormField>
    <MaterialWorkContext plan={plan} task={task} setPlan={setPlan} setTask={setTask} disabled={busy || !!versions.length}/>
    {error && <p role="alert">{error}</p>}
    {error&&versions.length>0&&<p className="judex-material-context-note">{t('matRegistrationRetry')}</p>}
  </UIDialog>;
}
