import {useRef, useState} from 'react';
import {Button} from '../../components/ui/Button';
import {FormField, UIOption, UISelect, UIWarning} from '../../components/ui/FormControls';
import {UIDialog} from '../../components/ui/Presentation';
import {useWork} from './store';
import type {Member, Position, Seat} from './types';

export const memberKey = (member: Member) => member.userId ?? member.name;
export const holdsPosition = (seat: Seat, member: Member) => seat.userId && member.userId ? seat.userId === member.userId : seat.person === member.name;

export function PositionAssignmentDialog({position, onClose}: {position: Position; onClose: () => void}) {
  const {state, project, t, text, act, setToast} = useWork();
  const [selected, setSelected] = useState('');
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  const inFlight = useRef(false);
  const assigned = state.seats.filter(seat => seat.positionId === position.id && (!seat.status || seat.status === 'active'));
  const available = project.members.filter(member => !assigned.some(seat => holdsPosition(seat, member)));
  const target = available.find(member => memberKey(member) === selected);
  const submit = async () => {
    if (!target || inFlight.current) return;
    inFlight.current = true; setBusy(true); setFailed(false);
    try {
      const result = await act('assignPositions', {projectId: project.id, person: target.name, userId: target.userId, ids: [position.id]}, {toast: false});
      if (result.ok) {setToast(t('teamAssignSuccess', {name: target.name, position: text(position.name)})); onClose();}
      else setFailed(true);
    } finally {inFlight.current = false; setBusy(false);}
  };
  return <UIDialog title={t('teamAssignTitle', {position: text(position.name)})} description={t('teamAssignHint')}
    onClose={onClose} dismissable={!busy} footer={<>
      <Button variant="ghost" disabled={busy} onPress={onClose}>{t('cancel')}</Button>
      <Button variant="primary" data-testid="assign-existing-position" disabled={!target || busy} onPress={() => void submit()}>{t(busy ? 'teamAssigning' : 'teamAssignConfirm')}</Button>
    </>}>
    <FormField label={t('teamAssignMember')}>
      <UISelect value={selected} disabled={busy || !available.length} data-testid="position-assignment-member" onChange={event => {setSelected(event.target.value); setFailed(false);}}>
        <UIOption value="">{t('assignmentTarget')}</UIOption>
        {available.map(member => <UIOption key={memberKey(member)} value={memberKey(member)}>{member.name}{member.email ? ' · ' + member.email : ''}</UIOption>)}
      </UISelect>
    </FormField>
    {!available.length && <p className="judex-muted">{t('teamAssignNoMembers')}</p>}
    <p className="judex-muted">{t('teamAssignBoundary')}</p>
    {failed && <UIWarning role="alert">{t('teamAssignFailed')}</UIWarning>}
  </UIDialog>;
}
