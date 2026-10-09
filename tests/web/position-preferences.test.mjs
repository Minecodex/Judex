import test from 'node:test';
import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {acceptInvite, personalPrompt} from '../../web/src/features/work/actions.ts';
import {myPositions, myPreference} from '../../web/src/features/work/preferences.ts';

test('one person keeps independent prompts and revisions for multiple positions', () => {
  let state = acceptInvite({...seedWork(), currentUser: '赵可'}, 'invite-zhao').state;
  const positions = myPositions(state, 'leaf');
  assert.equal(positions.length, 2);
  const [first,second] = positions;
  state = personalPrompt(state, 'leaf', first.id, 0, '开发偏好').state;
  state = personalPrompt(state, 'leaf', second.id, 0, '评审偏好').state;
  state = personalPrompt(state, 'leaf', first.id, 1, '').state;
  assert.equal(myPreference(state, 'leaf', first.id).prompt, '');
  assert.equal(myPreference(state, 'leaf', first.id).revision, 2);
  assert.equal(myPreference(state, 'leaf', second.id).prompt, '评审偏好');
  assert.equal(myPreference(state, 'leaf', second.id).revision, 1);
  assert.equal(personalPrompt(state, 'leaf', first.id, 1, '旧草稿').error, 'stale');
  assert.equal(personalPrompt(state, 'wild', first.id, 0, '跨项目').error, 'permission');
  assert.equal(personalPrompt(state, 'leaf', 'review-role', 0, '未任职').error, 'permission');
});

test('same names cannot expose another person\'s positions or preferences in API state', () => {
  const state = {...seedWork(), currentUser:'Same Name', currentUserId:'user-a'};
  state.positions = [{id:'a',projectId:'leaf'},{id:'b',projectId:'leaf'}];
  state.seats = [{id:'seat-a',positionId:'a',person:'Same Name',userId:'user-a',status:'active'},
    {id:'seat-b',positionId:'b',person:'Same Name',userId:'user-b',status:'active'}];
  state.preferences = [{projectId:'leaf',positionId:'a',person:'Same Name',userId:'user-b',prompt:'PRIVATE'}];
  assert.deepEqual(myPositions(state,'leaf').map(p=>p.id),['a']);
  assert.equal(myPreference(state,'leaf','a'),undefined);
  state.seats[0].status='suspended';
  assert.deepEqual(myPositions(state,'leaf'),[]);
});
