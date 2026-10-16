import {Plus,Trash2} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {FormField,UISelect,UIOption} from '../../components/ui/FormControls';
import {WorkflowTargetPicker} from './WorkflowTargetPicker';
import {useWork} from './store';
import type {FlowBody} from './types';

type Rule = NonNullable<FlowBody['hardRules']>[number];
export function WorkflowHardRulesEditor({body,onChange,disabled}:{body:FlowBody;onChange:(rules:Rule[])=>void;disabled:boolean}) {
  const {t} = useWork(),rules=body.hardRules ?? [];
  const update=(index:number,patch:Partial<Rule>)=>onChange(rules.map((rule,i)=>i===index?{...rule,...patch}:rule));
  return <section data-testid="workflow-hard-rules" className="judex-workflow-hard-rules">
    <div className="judex-workflow-draft-heading"><h3>{t('workflowHardRules')}</h3>
      <Button size="sm" disabled={disabled} data-testid="workflow-add-hard-rule" onPress={()=>onChange([...rules,{kind:'material_ready',phase:'start',targetId:''}])}><Plus/>{t('workflowHardRuleAdd')}</Button></div>
    <p className="judex-work-small-note">{t('workflowHardRuleHint')}</p>
    {rules.map((rule,index)=><div key={index} className="judex-workflow-hard-rule" data-testid={'workflow-hard-rule-'+index}>
      <FormField label={t('workflowHardRuleKind')}><UISelect value={rule.kind} disabled={disabled} data-testid={'workflow-hard-rule-kind-'+index} onChange={event=>update(index,{kind:event.target.value as Rule['kind'],targetId:''})}>
        <UIOption value="task_acceptance">{t('workflowHardRuleTask')}</UIOption><UIOption value="handoff_receipt">{t('workflowHardRuleReceipt')}</UIOption><UIOption value="material_ready">{t('workflowHardRuleMaterial')}</UIOption>
      </UISelect></FormField>
      <FormField label={t('workflowHardRulePhase')}><UISelect value={rule.phase} disabled={disabled} onChange={event=>update(index,{phase:event.target.value as Rule['phase']})}>
        <UIOption value="start">{t('workflowHardRuleStart')}</UIOption><UIOption value="accept">{t('workflowHardRuleAccept')}</UIOption><UIOption value="both">{t('workflowHardRuleBoth')}</UIOption>
      </UISelect></FormField>
      <FormField label={t('workflowHardRuleNode')}><UISelect value={rule.nodeId ?? ''} disabled={disabled} onChange={event=>update(index,{nodeId:event.target.value || undefined})}>
        <UIOption value="">{t('workflowHardRuleAllNodes')}</UIOption>{body.nodes.map(node=><UIOption key={node.id} value={node.id}>{node.name}</UIOption>)}
      </UISelect></FormField>
      <FormField label={t('workflowHardRuleTarget')}><WorkflowTargetPicker key={rule.kind} kind={rule.kind} value={rule.targetId ?? ''} disabled={disabled}
        testId={'workflow-hard-rule-target-'+index} onChange={targetId=>update(index,{targetId})}/></FormField>
      <Button size="sm" variant="danger-soft" disabled={disabled} data-testid={'workflow-remove-hard-rule-'+index} onPress={()=>onChange(rules.filter((_,i)=>i!==index))}><Trash2/>{t('workflowHardRuleRemove')}</Button>
    </div>)}
  </section>;
}
