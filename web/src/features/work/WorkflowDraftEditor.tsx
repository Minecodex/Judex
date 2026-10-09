import {useRef,useState} from 'react';
import {Plus,Save,Check,Trash2} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {FormField,UIInput,UITextArea,UICheckbox,UIDisclosure,UIWarning,UISelect,UIOption} from '../../components/ui/FormControls';
import {useWork} from './store';
import type {Flow} from './types';
import {bodyFromFlow,diagramFromBody,validFlowBody,feedbackCandidates} from './workflowPresets';
import {WorkflowDiagram} from './WorkflowDiagram';
import {uid} from './seed';

export function WorkflowDraftEditor({flow}: {flow:Flow}) {
  const {locale,t,state,project,act,management} = useWork();
  const [body,setBody] = useState(() => bodyFromFlow(flow,locale === 'en'));
  const [expectedVersion,setExpectedVersion] = useState(flow.definitionVersion ?? flow.version);
  const [busy,setBusy] = useState(false), [failed,setFailed] = useState(false);
  const inFlight = useRef(false);
  const saved = flow.draft?.body;
  const dirty = !saved || JSON.stringify(saved) !== JSON.stringify(body);
  const valid = validFlowBody(body);
  const stale = expectedVersion !== (flow.definitionVersion ?? flow.version);
  const perform = async (publish:boolean) => {
    if (inFlight.current || !management || !valid || (publish && (dirty || !flow.draft))) return;
    inFlight.current=true;setBusy(true);setFailed(false);
    try {
      const result = publish ? await act('publishFlowDraft',{projectId:project.id,flowId:flow.id,
        expectedVersion,draftHash:flow.draft!.hash}) :
        await act('saveFlowDraft',{projectId:project.id,flowId:flow.id,expectedVersion,body});
      if (!result.ok) setFailed(true);
      else setExpectedVersion(version => version+1);
    } finally {inFlight.current=false;setBusy(false);}
  };
  const updateNode = (id:string,field:'name'|'responsibility'|'phase'|'kind',value:string) =>
    setBody(current => ({...current,nodes:current.nodes.map(n => n.id === id ? {...n,[field]:value}:n)}));
  return <div className="judex-workflow-draft-editor" data-testid="workflow-draft-editor">
    <p className="judex-work-small-note">{t('workflowDraftBinding')}</p>
    <FormField label={t('workflowDraftName')}><UIInput disabled={busy || !management} value={body.name} data-testid="workflow-draft-name"
      onChange={event => setBody(current => ({...current,name:event.target.value}))}/></FormField>
    <FormField label={t('workflowDraftInstructions')}><UITextArea disabled={busy || !management} value={body.instructions ?? ''} data-testid="workflow-draft-instructions"
      onChange={event => setBody(current => ({...current,instructions:event.target.value}))}/></FormField>
    <WorkflowDiagram flow={diagramFromBody(body)}/>
    <div className="judex-workflow-draft-heading"><h3>{t('workflowDraftNodes')}</h3>
      {management && <Button size="sm" disabled={busy} data-testid="workflow-add-node" onPress={() => setBody(current => ({...current,
        nodes:[...current.nodes,{id:'node'+uid().replaceAll('-',''),name:t('workflowDraftNewNode'),responsibility:'',allowedPositionIds:[],defaultApprovalPolicy:'all'}]}))}><Plus/>{t('workflowDraftAddNode')}</Button>}</div>
    {body.nodes.map((node,index) => {
      const bound = state.positions.some(p => p.projectId === project.id && p.bindings.some(b => b.flowId === flow.id && b.nodeId === node.id));
      const protectedNode = bound || !!node.allowedPositionIds?.length || !!node.delegationUserIds?.length ||
        !!body.hardRules?.some(r => r.nodeId === node.id);
      return <UIDisclosure key={node.id} className="judex-workflow-node-editor" title={(index+1)+'. '+node.name}>
        <FormField label={t('workflowDraftPhase')}><UIInput value={node.phase ?? ''} disabled={busy || !management} maxLength={120} data-testid={'workflow-node-phase-'+node.id}
          onChange={event => updateNode(node.id,'phase',event.target.value)}/></FormField>
        <FormField label={t('workflowDraftKind')}><UISelect value={node.kind ?? 'activity'} disabled={busy || !management} aria-label={t('workflowDraftKind')}
          onChange={event => setBody(current => ({...current,nodes:current.nodes.map(n => n.id === node.id ? {...n,kind:event.target.value as 'activity'|'decision'}:n)}))}>
          <UIOption value="activity">{t('workflowDraftActivity')}</UIOption><UIOption value="decision">{t('workflowDraftDecision')}</UIOption>
        </UISelect></FormField>
        <FormField label={t('workflowDraftNodeName')}><UIInput disabled={busy || !management} value={node.name} data-testid={'workflow-node-name-'+node.id}
          onChange={event => updateNode(node.id,'name',event.target.value)}/></FormField>
        <FormField label={t('workflowDraftResponsibility')}><UITextArea disabled={busy || !management} value={node.responsibility} data-testid={'workflow-node-responsibility-'+node.id}
          onChange={event => updateNode(node.id,'responsibility',event.target.value)}/></FormField>
        <fieldset className="judex-workflow-link-options"><legend>{t('workflowDraftLinks')}</legend>{body.nodes.filter(n => n.id !== node.id).map(target =>
          <UICheckbox key={target.id} disabled={busy || !management} checked={!!body.advisoryEdges?.some(e => e.kind !== 'feedback' && e.from === node.id && e.to === target.id)}
            onChange={event => setBody(current => ({...current,advisoryEdges:event.target.checked ?
              [...current.advisoryEdges ?? [],{from:node.id,to:target.id}] :
              (current.advisoryEdges ?? []).filter(e => e.kind === 'feedback' || e.from !== node.id || e.to !== target.id)}))}>{target.name}</UICheckbox>)}</fieldset>
        {node.kind === 'decision' && <fieldset className="judex-workflow-link-options"><legend>{t('workflowDraftFeedback')}</legend>{feedbackCandidates(body,node.id).map(target => {
          const edge=body.advisoryEdges?.find(e => e.kind === 'feedback' && e.from === node.id && e.to === target.id);
          return <div className="judex-workflow-return-option" key={target.id}>
            <UICheckbox disabled={busy || !management} checked={!!edge} onChange={event => setBody(current => ({...current,
              advisoryEdges:event.target.checked ? [...current.advisoryEdges ?? [],{from:node.id,to:target.id,kind:'feedback',label:t('workflowDraftDefaultReturn')}] :
                (current.advisoryEdges ?? []).filter(e => e.kind !== 'feedback' || e.from !== node.id || e.to !== target.id)}))}>{target.name}</UICheckbox>
            {edge && <UIInput value={edge.label ?? ''} disabled={busy || !management} aria-label={t('workflowDraftReturnLabel')} maxLength={200}
              onChange={event => setBody(current => ({...current,advisoryEdges:(current.advisoryEdges ?? []).map(e =>
                e.kind === 'feedback' && e.from === node.id && e.to === target.id ? {...e,label:event.target.value}:e)}))}/>}
          </div>;
        })}</fieldset>}
        {management && <Button size="sm" variant="danger-soft" disabled={busy || protectedNode || body.nodes.length < 2} onPress={() => setBody(current => {
          const policies = {...current.approvalPolicies};delete policies[node.id];
          return {...current,nodes:current.nodes.filter(n => n.id !== node.id),advisoryEdges:(current.advisoryEdges ?? []).filter(e => e.from !== node.id && e.to !== node.id),approvalPolicies:policies};
        })}><Trash2/>{t('workflowDraftRemoveNode')}</Button>}
        {protectedNode && <p className="judex-work-small-note">{t('workflowDraftProtectedNode')}</p>}
      </UIDisclosure>;
    })}
    {!valid && <UIWarning role="alert">{t('workflowDraftInvalid')}</UIWarning>}
    {stale && <UIWarning role="alert">{t('workflowDraftStale')}<Button size="sm" disabled={busy} onPress={() => {
      setBody(bodyFromFlow(flow,locale === 'en'));setExpectedVersion(flow.definitionVersion ?? flow.version);setFailed(false);
    }}>{t('workflowDraftReload')}</Button></UIWarning>}
    {failed && <UIWarning role="alert">{t('workflowDraftFailed')}</UIWarning>}
    {management && <div className="judex-workflow-draft-actions">
      <Button variant="secondary" disabled={busy || !dirty || !valid} data-testid="save-workflow-draft" onPress={() => void perform(false)}><Save/>{t('workflowDraftSave')}</Button>
      <Button variant="primary" disabled={busy || stale || dirty || !flow.draft || !valid} data-testid="publish-workflow-draft" onPress={() => void perform(true)}><Check/>{t('workflowDraftPublish')}</Button>
    </div>}
    {management && dirty && <p className="judex-work-small-note">{t('workflowDraftUnsaved')}</p>}
  </div>;
}
