import type { Position } from './types';
import { useWork } from './store';
import { Btn, Field } from './ui';
import { UISelect, UIOption } from '../../components/ui/FormControls';

export function PositionBindingsEditor({ bindings, onChange }: {
  bindings: Position['bindings']; onChange: (bindings: Position['bindings']) => void;
}) {
  const { state, project, t, text } = useWork();
  const flows = state.flows.filter(f => f.projectId === project.id && f.status !== 'draft');
  const update = (index: number, binding: Position['bindings'][number]) =>
    onChange(bindings.map((b, i) => i === index ? binding : b));
  return <div className="judex-position-bindings">
    {!flows.length && <p className="judex-muted">{t('positionPresetsNoFlow')}</p>}
    {bindings.map((binding, index) => {
      const flow = flows.find(f => f.id === binding.flowId);
      return <div key={index} className="judex-position-binding" data-testid={`position-binding-${index}`}>
        <div className="judex-work-form-grid">
          <Field label={t('workChooseFlow')}>
            <UISelect className="judex-input" data-testid={index ? `position-flow-${index}` : 'position-flow'}
              value={binding.flowId} onChange={e => {
                const selected = flows.find(f => f.id === e.target.value);
                update(index, {flowId: selected?.id ?? '', nodeId: selected?.nodes[0]?.id ?? ''});
              }}>
              <UIOption value="">{t('positionPresetsNoBinding')}</UIOption>
              {!flow && binding.flowId && <UIOption value={binding.flowId}>{t('workBindingUnavailable')}</UIOption>}
              {flows.map(f => <UIOption key={f.id} value={f.id}>{text(f.name)}</UIOption>)}
            </UISelect>
          </Field>
          <Field label={t('workChooseNode')}>
            <UISelect className="judex-input" data-testid={index ? `position-node-${index}` : 'position-node'}
              value={binding.nodeId} disabled={!flow}
              onChange={e => update(index, {...binding, nodeId: e.target.value})}>
              <UIOption value="">{t('positionPresetsNoBinding')}</UIOption>
              {flow?.nodes.map(n => <UIOption key={n.id} value={n.id}>{text(n.label)}</UIOption>)}
            </UISelect>
          </Field>
        </div>
        <Btn secondary testId={`position-binding-remove-${index}`}
          onClick={() => onChange(bindings.filter((_, i) => i !== index))}>{t('workRemoveBinding')}</Btn>
      </div>;
    })}
    <Btn secondary testId="position-binding-add" disabled={!flows.length}
      onClick={() => onChange([...bindings, {flowId: '', nodeId: ''}])}>{t('workAddBinding')}</Btn>
  </div>;
}
