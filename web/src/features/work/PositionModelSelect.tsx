import {useQuery} from '@tanstack/react-query';
import {requestAll} from '../../lib/api/client';
import type {components} from '../../lib/api/schema';
import {UISelect, UIOption} from '../../components/ui/FormControls';
import {Field} from './ui';
import {useWork} from './store';

type Model = components['schemas']['ModelSummary'];
export function PositionModelSelect({value, onChange}: {value: string | null; onChange: (value: string | null) => void}) {
  const {t} = useWork();
  const query = useQuery({queryKey:['modelCatalog'], queryFn: ({signal}) => requestAll<Model>('/models', {signal})});
  const models = (query.data ?? []).filter(m => m.enabled && m.capabilities.toolCalling && m.capabilities.streaming);
  return <Field label={t('workPositionModel')}>
    <UISelect aria-label={t('workPositionModel')} data-testid="position-model" value={value ?? ''}
      onChange={e => onChange(e.target.value || null)} disabled={query.isPending || query.isError}>
      <UIOption value="">{t('workInheritProjectModel')}</UIOption>
      {value && !models.some(m => m.id === value) && <UIOption value={value}>{t('workModelUnavailable')}</UIOption>}
      {models.map(m => <UIOption key={m.id} value={m.id}>{m.displayName}</UIOption>)}
    </UISelect>
    {query.isError && <p className="judex-muted">{t('workModelLoadFailed')}</p>}
  </Field>;
}
