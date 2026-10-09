import {myPositions} from "./preferences";
import {Bot, SlidersHorizontal, UserPlus} from 'lucide-react';
import {Button} from '../../components/ui/Button';
import {CardActions} from '../../components/ui/ActionGroup';
import {UICard, UIStatus} from '../../components/ui/FormControls';
import {PersonAvatar} from '../../components/ui/Presentation';
import {useWork} from './store';
import type {Position} from './types';

export function PositionCard({position, onEdit, onAssign,onPreferences}: {position: Position; onEdit: () => void; onAssign: () => void;onPreferences?:()=>void}) {
  const {state, project, text, t, management, go} = useWork();
  const assigned = state.seats.filter(seat => seat.positionId === position.id && seat.person && (!seat.status || seat.status === 'active'));
  return <UICard className="judex-position-card" data-testid={'position-card-' + position.id}>
    <div className="judex-position-card-top"><span className={'judex-position-symbol judex-tone-' + position.tone}><Bot/></span>
      {!position.bindings.length && <UIStatus>{t('positionPresetsUnbound')}</UIStatus>}</div>
    <h3>{text(position.name)}</h3>
    <p>{text(position.publicSummary && text(position.publicSummary) ? position.publicSummary : position.prompt)}</p>
    {!!position.bindings.length && <div className="judex-position-nodes">{position.bindings.map(binding => {
      const flow = state.flows.find(item => item.id === binding.flowId);
      const node = flow?.nodes.find(item => item.id === binding.nodeId);
      return <Button size="sm" key={binding.flowId + binding.nodeId} onPress={() => go({view: 'flows', id: binding.flowId})}>
        {flow && node ? text(flow.name) + ' / ' + text(node.label) : t('teamBindingReview')}
      </Button>;
    })}</div>}
    <div className="judex-position-holders"><span>{t('teamPositionMembers')}</span>
      {assigned.length ? <div>{assigned.map(seat => {
        const member = project.members.find(item => seat.userId ? item.userId === seat.userId : item.name === seat.person);
        return <span key={seat.id} className="judex-position-holder" title={member?.email ?? seat.person}><PersonAvatar name={seat.person} small/>{seat.person}</span>;
      })}</div> : <small>{t('teamPositionUnassigned')}</small>}
    </div>
    {management && <CardActions><Button variant="outline" onPress={onEdit} data-testid={'edit-position-' + position.id}><SlidersHorizontal/>{t('workEditPosition')}</Button>
      <Button variant="primary" onPress={onAssign} data-testid={'assign-position-' + position.id}><UserPlus/>{t('teamAssignOpen')}</Button></CardActions>}
    {onPreferences&&myPositions(state,project.id).some(p=>p.id===position.id)&&<CardActions><Button variant="outline" data-testid={'position-preferences-'+position.id} onPress={onPreferences}>{t('coopPrivateRolePreferences')}</Button></CardActions>}
  </UICard>;
}
