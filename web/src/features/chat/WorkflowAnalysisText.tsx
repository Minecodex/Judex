import type {components} from '../../lib/api/schema';
import {UIWarning} from '../../components/ui/FormControls';
import {useWork} from '../work/store';

type Output = components['schemas']['TaskAnalysisOutput'];
export function WorkflowAnalysisText({output}: {output: Output}) {
  const {state,t,text} = useWork();
  const groups = [
    {label:t('coCurrentNodes'),nodes:output.currentNodeCandidates},
    {label:t('coRelatedNodes'),nodes:output.relatedEvidenceNodes},
    {label:t('coNextNodes'),nodes:output.suggestedNextNodes},
  ];
  return <>
    {output.needsClarification && <UIWarning>{t('coNeedsClarification')}</UIWarning>}
    {groups.map(group => !!group.nodes?.length && <div key={group.label}>
      <strong>{group.label}</strong>
      {group.nodes.map(node => {
        const flow = state.flows.find(f => f.id === node.workflowId);
        const label = flow?.nodes.find(n => n.id === node.nodeId)?.label;
        return <p key={`${node.workflowId}/${node.versionId}/${node.nodeId}`}>
          {flow ? text(flow.name) : node.workflowId} · {label ? text(label) : node.nodeId}: {node.reason}
        </p>;
      })}
    </div>)}
  </>;
}
