import {useRef,useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Layers,Maximize2,Sparkles,ExternalLink} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {FormField,UICheckbox,UIOption,UISelect,UIStatus,UIWarning} from '../../components/ui/FormControls';
import {EmptyState,UIDialog} from '../../components/ui/Presentation';
import {dataMode,request} from '../../lib/api/client';
import {useWork} from './store';
import {builtinWorkflowPresets,existingPresetWorkflow,workflowPresetBody,diagramFromBody,type WorkflowPresetCatalog,type WorkflowPreset} from './workflowPresets';
import {WorkflowDiagram,WorkflowViewer} from './WorkflowDiagram';

export function WorkflowPresetsDialog({onClose}: {onClose:()=>void}) {
  const {state,project,t,text,locale,act,setToast,go} = useWork();
  const [scenarioId,setScenario] = useState(''), [selection,setSelection] = useState<string[]>([]);
  const [submitting,setSubmitting] = useState(false), [failed,setFailed] = useState(false), [preview,setPreview] = useState<WorkflowPreset|null>(null);
  const inFlight = useRef(false);
  const catalog = useQuery({queryKey:['workflow-presets',dataMode],staleTime:300000,retry:false,
    queryFn:() => dataMode === 'demo' ? Promise.resolve(builtinWorkflowPresets) : request<WorkflowPresetCatalog>('/workflow-presets')});
  const scenario = catalog.data?.scenarios.find(s => s.id === scenarioId);
  const flows = state.flows.filter(f => f.projectId === project.id);
  const templates = scenario?.workflowIds.flatMap(id => catalog.data?.workflows.find(p => p.id === id) ?? []) ?? [];
  const available = templates.filter(p => !existingPresetWorkflow(flows,p)).map(p => p.id);
  const selected = selection.filter(id => available.includes(id));
  const allSelected = available.length > 0 && available.length === selected.length;
  const submit = async () => {
    if (!catalog.data || !scenario || !selected.length || inFlight.current) return;
    inFlight.current = true;setSubmitting(true);setFailed(false);
    try {
      const result = await act('importWorkflowPresets',{projectId:project.id,catalogVersion:catalog.data.version,scenarioId,workflowIds:selected,locale},{toast:false});
      if (result.ok && result.createdCount !== undefined) {
        setToast(t('workflowPresetsSuccess',{count:result.createdCount})+(result.skippedCount ? t('workflowPresetsSkipped',{count:result.skippedCount}):''));
        onClose();if(result.id)go({view:'flows',id:result.id});
      } else {setFailed(true);void catalog.refetch();}
    } finally {inFlight.current=false;setSubmitting(false);}
  };
  const previewAdded = preview && existingPresetWorkflow(flows,preview);
  const footer = preview ? <div className="judex-preset-footer">
    <span>{previewAdded ? t('workflowPresetsAdded'):t('workflowPresetsBoundary')}</span><div>
      <Button variant="ghost" data-testid="workflow-preview-back" onPress={() => setPreview(null)}>{t('workflowPresetsBack')}</Button>
      <Button variant="primary" disabled={!!previewAdded} data-testid="workflow-preview-select" onPress={() => {
        setSelection(current => current.includes(preview.id) ? current : [...current,preview.id]);setPreview(null);
      }}>{t(selected.includes(preview.id)?'workflowPresetsChosen':'workflowPresetsSelect')}</Button>
    </div></div> : <div className="judex-preset-footer">
    <span role="status" data-testid="workflow-preset-selection-count">{t('workflowPresetsSelected',{count:selected.length})}</span><div>
      <Button variant="ghost" disabled={submitting} onPress={onClose}>{t('cancel')}</Button>
      <Button variant="primary" disabled={!selected.length || submitting || catalog.isError} data-testid="import-workflow-presets" onPress={() => void submit()}>
        {t(submitting?'workflowPresetsAdding':'workflowPresetsAdd',{count:selected.length})}</Button>
    </div></div>;
  return <UIDialog title={preview?text(preview.name):t('workflowPresetsTitle')} description={preview?text(preview.summary):t('workflowPresetsHint')}
    onClose={() => preview ? setPreview(null):onClose()} wide fullscreen={!!preview} dismissable={!submitting} footer={footer}>
    {preview ? <div data-testid="workflow-preset-full-preview">
      <WorkflowViewer flow={diagramFromBody(workflowPresetBody(preview,locale))}/>
      {!!preview.sources?.length && <div className="judex-workflow-source-links" data-testid="workflow-source-links">
        <strong>{t('workflowSources')}</strong>{preview.sources.map(source => <a key={source.url} href={source.url} target="_blank" rel="noopener noreferrer">{text(source.title)}<ExternalLink/></a>)}
      </div>}
      <p className="judex-workflow-suggested-roles">{t('workflowPresetsRoles')}：{preview.roleIds.map(id => catalog.data?.roles.find(r => r.id === id)).filter(r => !!r).map(r => text(r!.name)).join(' · ')}</p>
      <div className="judex-workflow-preview-notes">{preview.nodes.map(n => <article key={n.id}>{n.phase && <small>{text(n.phase)}</small>}<h3>{text(n.name)}</h3><p>{text(n.responsibility)}</p></article>)}</div>
    </div> : <div className="judex-workflow-presets" data-testid="workflow-presets-dialog">
      {catalog.isPending && <p role="status">{t('workflowPresetsLoading')}</p>}
      {catalog.isError && <UIWarning role="alert">{t('workflowPresetsLoadError')}<Button size="sm" onPress={() => void catalog.refetch()}>{t('workflowPresetsRetry')}</Button></UIWarning>}
      {catalog.data && <>
        <div className="judex-preset-scenario"><span className="judex-preset-scenario-icon"><Layers/></span><div>
          <FormField label={t('workflowPresetsScenario')}><UISelect value={scenarioId} disabled={submitting} data-testid="workflow-preset-scenario" aria-label={t('workflowPresetsScenario')}
            onChange={event => {setScenario(event.target.value);setSelection([]);setFailed(false);}}>
            <UIOption value="">{t('workflowPresetsChoose')}</UIOption>{catalog.data.scenarios.map(s => <UIOption key={s.id} value={s.id}>{text(s.name)}</UIOption>)}
          </UISelect></FormField>
        </div></div>
        {scenario ? <>
          <div className="judex-preset-toolbar"><div><h3>{text(scenario.name)}</h3><p>{text(scenario.description)} · {t('workflowPresetsCount',{count:templates.length})}</p></div>
            <Button size="sm" variant="ghost" disabled={submitting || !available.length} data-testid="workflow-preset-select-all"
              onPress={() => setSelection(allSelected?[]:available)}>{t(allSelected?'workflowPresetsClear':'workflowPresetsAll')}</Button></div>
          <div className="judex-preset-grid">{templates.map(p => {
            const added = existingPresetWorkflow(flows,p),checked = selected.includes(p.id);
            return <div key={p.id} className="judex-workflow-choice" data-testid={'workflow-preset-card-'+p.id}>
              <UICheckbox appearance="card" className={'judex-workflow-choice-control'+(added?' judex-preset-added':'')} checked={checked} disabled={!!added || submitting}
                aria-label={text(p.name)} onChange={event => setSelection(current => event.target.checked ? [...current,p.id]:current.filter(id => id !== p.id))}>
                <span className="judex-workflow-choice-content"><span className="judex-workflow-choice-title"><strong>{text(p.name)}</strong><span className="judex-workflow-choice-badges">
                  <UIStatus>{t(p.complexity === 'advanced'?'workflowAdvanced':'workflowBasic')}</UIStatus>{added && <UIStatus>{t('workflowPresetsAdded')}</UIStatus>}</span></span>
                  <WorkflowDiagram flow={diagramFromBody(workflowPresetBody(p,locale))} thumbnail/>
                  <span className="judex-preset-summary">{text(p.summary)}</span><span className="judex-workflow-node-count">{t('workflowPresetsNodes',{count:p.nodes.length})}</span>
                </span>
              </UICheckbox>
              <Button size="sm" className="judex-workflow-preview-action" disabled={submitting} data-testid={'preview-workflow-'+p.id} onPress={() => setPreview(p)}><Maximize2/>{t('workflowPresetsPreview')}</Button>
            </div>;
          })}</div>
        </> : <EmptyState title={t('workflowPresetsChoose')} description={t('workflowPresetsChooseHint')} icon={<Sparkles/>}/>}
      </>}
      <p className="judex-preset-boundary">{t('workflowPresetsBoundary')}</p>
      {failed && <UIWarning role="alert">{t('workflowPresetsFailed')}</UIWarning>}
    </div>}
  </UIDialog>;
}
