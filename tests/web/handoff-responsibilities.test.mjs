import test from 'node:test';import assert from 'node:assert/strict';
import {seedWork} from '../../web/src/features/work/seed.ts';
import {defaultHandoffSender,handoffResponsibilities} from '../../web/src/features/work/handoffResponsibilities.ts';
import {proposeHandoff} from '../../web/src/features/work/composition.ts';

test('a shared task uses the acting member responsibility and leaves multiple held duties unresolved',()=>{
 const s=seedWork();s.currentUser='顾言';const task=s.tasks.find(t=>t.id==='build');task.seatIds=['lead','maker'];
 assert.equal(defaultHandoffSender(s,task),'maker');
 s.seats.push({...s.seats.find(v=>v.id==='maker'),id:'maker-two'});task.seatIds.push('maker-two');assert.equal(defaultHandoffSender(s,task),'');
 task.status='delivered';task.files=[{id:'proof',name:'proof',text:'proof',at:1,author:'顾言'}];
 assert.equal(proposeHandoff(s,'leaf',task.id,task.reviewerSeatId,[task.id],'stage').error,'required');
 const result=proposeHandoff(s,'leaf',task.id,task.reviewerSeatId,[task.id],'stage',{[task.id]:'maker-two'});assert.equal(result.state.handoffs.at(-1).sources[0].senderSeatId,'maker-two');
 assert.equal(proposeHandoff(s,'leaf',task.id,task.reviewerSeatId,[task.id],'stage',{[task.id]:'foreign'}).error,'required');
});
test('source duty selection excludes inactive, unbound and other-project identities while allowing explicit peer coordination',()=>{
 const s=seedWork();s.currentUser='顾言';s.seats.find(v=>v.id==='maker').status='retired';
 assert.equal(handoffResponsibilities(s,'leaf').includes('maker'),false);const role=s.seats.find(v=>v.id==='lead');assert.equal(handoffResponsibilities(s,'leaf').includes(role.id),true);
 const task=s.tasks.find(v=>v.id==='build');task.status='delivered';task.files=[{id:'proof',name:'proof',text:'proof',at:1,author:'顾言'}];
 const coordinated=proposeHandoff(s,'leaf',task.id,task.reviewerSeatId,[task.id],'stage',{[task.id]:role.id});assert.equal(coordinated.state.handoffs.at(-1).sources[0].senderSeatId,role.id);
 const other=s.seats.find(v=>!handoffResponsibilities(s,'leaf').includes(v.id)&&v.id!=='maker');if(other)assert.equal(proposeHandoff(s,'leaf',task.id,task.reviewerSeatId,[task.id],'stage',{[task.id]:other.id}).error,'required');
});
