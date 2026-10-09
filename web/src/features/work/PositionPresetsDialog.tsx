import { useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { BriefcaseBusiness, Layers, Sparkles } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { FormField, UICheckbox, UIOption, UISelect, UIStatus, UIWarning } from '../../components/ui/FormControls';
import { EmptyState, UIDialog } from '../../components/ui/Presentation';
import { dataMode, request } from '../../lib/api/client';
import { useWork } from './store';
import { builtinPositionPresets, existingPresetPosition, type PositionPresetCatalog } from './positionPresets';

export function PositionPresetsDialog({onClose}: {onClose: () => void}) {
  const {state, project, t, text, locale, act, setToast} = useWork();
  const [scenarioId, setScenarioId] = useState('');
  const [selection, setSelection] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [failed, setFailed] = useState(false);
  const inFlight = useRef(false);
  const catalog = useQuery({queryKey: ['position-presets', dataMode], staleTime: 300000, retry: false,
    queryFn: () => dataMode === 'demo' ? Promise.resolve(builtinPositionPresets) : request<PositionPresetCatalog>('/position-presets')});
  const scenario = catalog.data?.scenarios.find(item => item.id === scenarioId);
  const positions = state.positions.filter(position => position.projectId === project.id);
  const roles = scenario?.roleIds.flatMap(id => catalog.data?.roles.find(role => role.id === id) ?? []) ?? [];
  const available = roles.filter(role => !existingPresetPosition(positions, role)).map(role => role.id);
  const selected = selection.filter(id => available.includes(id));
  const allSelected = available.length > 0 && selected.length === available.length;
  const submit = async () => {
    if (!catalog.data || !scenario || !selected.length || inFlight.current) return;
    inFlight.current = true;
    setSubmitting(true);
    setFailed(false);
    try {
      const result = await act('importPositionPresets', {projectId: project.id, catalogVersion: catalog.data.version,
        scenarioId: scenario.id, roleIds: selected, locale}, {toast: false});
      if (result.ok && result.createdCount !== undefined) {
        setToast(t('positionPresetsSuccess', {count: result.createdCount}) +
          (result.skippedCount ? t('positionPresetsSkipped', {count: result.skippedCount}) : ''));
        onClose();
      } else {
        setFailed(true);
        void catalog.refetch();
      }
    } finally {
      inFlight.current = false;
      setSubmitting(false);
    }
  };
  return <UIDialog title={t('positionPresetsTitle')} description={t('positionPresetsHint')} onClose={onClose}
    wide dismissable={!submitting} footer={<div className="judex-preset-footer">
      <span role="status" data-testid="preset-selection-count">{t('positionPresetsSelected', {count: selected.length})}</span>
      <div><Button variant="ghost" disabled={submitting} onPress={onClose}>{t('cancel')}</Button>
        <Button variant="primary" data-testid="import-position-presets" disabled={!selected.length || submitting || catalog.isError}
          onPress={() => void submit()}>{t(submitting ? 'positionPresetsAdding' : 'positionPresetsAdd', {count: selected.length})}</Button></div>
    </div>}>
    <div className="judex-position-presets" data-testid="position-presets-dialog">
      {catalog.isPending && <p role="status" className="judex-muted">{t('positionPresetsLoading')}</p>}
      {catalog.isError && <UIWarning role="alert">{t('positionPresetsLoadError')}<Button size="sm" onPress={() => void catalog.refetch()}>{t('positionPresetsRetry')}</Button></UIWarning>}
      {catalog.data && <>
        <div className="judex-preset-scenario"><span className="judex-preset-scenario-icon"><Layers/></span><div>
          <FormField label={t('positionPresetsScenario')}>
          <UISelect aria-label={t('positionPresetsScenario')} data-testid="position-preset-scenario" disabled={submitting}
            value={scenarioId} onChange={event => {setScenarioId(event.target.value); setSelection([]); setFailed(false);}}>
            <UIOption value="">{t('positionPresetsChoose')}</UIOption>
            {catalog.data.scenarios.map(item => <UIOption key={item.id} value={item.id}>{text(item.name)}</UIOption>)}
          </UISelect>
          </FormField>
        </div></div>
        {scenario ? <>
          <div className="judex-preset-toolbar"><div><h3>{text(scenario.name)}</h3><p>{text(scenario.description)}</p></div>
            <Button size="sm" variant="ghost" disabled={submitting || !available.length} data-testid="preset-select-all"
              onPress={() => setSelection(allSelected ? [] : available)}>{t(allSelected ? 'positionPresetsClear' : 'positionPresetsSelectAll')}</Button>
          </div>
          <fieldset className="judex-preset-grid"><legend className="judex-preset-legend">{t('positionPresetsRoles')} · {t('positionPresetsCount', {count: roles.length})}</legend>
            {roles.map(role => {
              const added = existingPresetPosition(positions, role);
              const checked = selected.includes(role.id);
              return <UICheckbox key={role.id} appearance="card" className={'judex-position-preset-card' + (checked ? ' judex-preset-selected' : '') + (added ? ' judex-preset-added' : '')}
                checked={checked} disabled={!!added || submitting} aria-label={text(role.name)} data-testid={'preset-card-' + role.id}
                onChange={event => setSelection(current => event.target.checked ? [...current, role.id] : current.filter(id => id !== role.id))}>
                <span className="judex-preset-card-content"><span className="judex-preset-card-top"><span className="judex-preset-role-icon"><BriefcaseBusiness/></span>
                  {added && <UIStatus>{t('positionPresetsAdded')}</UIStatus>}</span>
                  <strong>{text(role.name)}</strong><span className="judex-preset-summary">{text(role.summary)}</span></span>
              </UICheckbox>;
            })}
          </fieldset>
        </> : <EmptyState title={t('positionPresetsChoose')} description={t('positionPresetsChooseHint')} icon={<Sparkles/>}/>}
      </>}
      <p className="judex-preset-boundary">{t('positionPresetsBoundary')}</p>
      {failed && <UIWarning role="alert">{t('positionPresetsFailed')}</UIWarning>}
    </div>
  </UIDialog>;
}
