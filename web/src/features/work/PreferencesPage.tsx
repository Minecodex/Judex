import { useState } from 'react';
import { Bot, LockKeyhole } from 'lucide-react';
import { UICard, UIOption, UISelect, UITextArea } from '../../components/ui/FormControls';
import { EmptyState } from '../../components/ui/Presentation';
import { useWorkspaceDraft } from '../chat/useWorkspaceDraft';
import { dataMode } from '../../lib/api/client';
import { useWork } from './store';
import { Btn, Field, Heading, Person } from './ui';
import { myPositions, myPreference } from './preferences';
import type { Position } from './types';

function PromptEditor({ position, saving, setSaving }: {
  position: Position; saving: boolean; setSaving: (saving: boolean) => void;
}) {
  const { state, project, t, act } = useWork();
  const preference = myPreference(state, project.id, position.id);
  const [draft, setDraft] = useWorkspaceDraft('personal-prompt:' + position.id, {
    prompt: preference?.prompt ?? '', revision: preference?.revision ?? 0,
  });
  return <>
    <Field label={t('workMyPositionPrompt')}>
      <UITextArea className="judex-textarea judex-prompt-editor" data-testid="next-personal-prompt"
        value={draft.prompt} disabled={saving} maxLength={20000}
        onChange={event => setDraft({ ...draft, prompt: event.target.value })}
        placeholder={t('workPromptExample')} />
    </Field>
    <Btn testId="next-save-preference" disabled={saving} onClick={async () => {
      setSaving(true);
      try {
        const result = await act('personalPrompt', { projectId: project.id, positionId: position.id,
          expectedRevision: draft.revision, prompt: draft.prompt });
        if (result.ok) setDraft({ ...draft, revision: draft.revision + 1 });
      } finally { setSaving(false); }
    }}>{t('personalSave')}</Btn>
  </>;
}

function ProjectPreferences({initialPositionId,embedded=false}:{initialPositionId?:string;embedded?:boolean}) {
  const { state, project, t, text, act, management, reset,go } = useWork();
  const positions = myPositions(state, project.id);
  const [selected, setSelected] = useWorkspaceDraft('personal-prompt-position', '');
  const [saving, setSaving] = useState(false), [resetting, setResetting] = useState(false);
  const position = positions.find(position => position.id === (embedded?initialPositionId:(selected||initialPositionId))) ?? positions[0];
  return <div className="judex-position-preferences">
    {embedded?<p className="judex-modal-description">{t('workMyPromptHint')}</p>:<Heading eyebrow="YOUR WAY OF WORKING" title={t('workMyPrompt')} description={t('workMyPromptHint')} />}
    <div className="judex-work-preferences">
      <UICard className="judex-work-panel">
        <Person name={state.currentUser} />
        {position ? <>
          <Field label={t('workMyPosition')}>
            <UISelect data-testid="personal-prompt-position" value={position.id} disabled={saving}
              onChange={event => embedded?go({settingsItem:"preference:"+event.target.value}):setSelected(event.target.value)}>
              {positions.map(position => <UIOption key={position.id} value={position.id}>{text(position.name)}</UIOption>)}
            </UISelect>
          </Field>
          <PromptEditor key={position.id} position={position} saving={saving} setSaving={setSaving} />
        </> : <EmptyState icon={<Bot />} title={t('workNoMyPositions')} description={t('workNoMyPositionsHint')} />}
      </UICard>
      <aside>
        <LockKeyhole /><h3>{t('personalPrivate')}</h3>
        <p>{t('personalHint')}</p><p>{t('promptBoundary')}</p>
      </aside>
    </div>
    {dataMode === 'demo' && !embedded && <div className="judex-next-demo-settings">
      <p>{t('workSimulated')}</p>
      {management && <Btn secondary onClick={() => void act('remindPending', {projectId: project.id})}
        testId="simulate-receipt-reminder">{t('workReminderDemo')}</Btn>}
      <p>{t('workResetHint')}</p>
      {resetting ? <div>
        <Btn danger testId="reset-next" onClick={reset}>{t('confirm')}</Btn>
        <Btn secondary onClick={() => setResetting(false)}>{t('cancel')}</Btn>
      </div> : <Btn secondary onClick={() => setResetting(true)}>{t('workReset')}</Btn>}
    </div>}
  </div>;
}

export function PreferencesPage({initialPositionId,embedded=false}:{initialPositionId?:string;embedded?:boolean}={}) {
  const { state, project } = useWork();
  return <ProjectPreferences initialPositionId={initialPositionId} embedded={embedded} key={project.id + ':' + (state.currentUserId ?? state.currentUser)} />;
}
