import test from 'node:test';import assert from 'node:assert/strict';import {overlappingFaults} from './fault-windows.mjs';
const epoch=Date.parse('2026-10-15T00:00:00Z'),windows=[{id:'restart',startedAt:new Date(epoch+1000).toISOString(),closedAt:new Date(epoch+5000).toISOString()}];
test('a request that returns after recovery still overlaps the interruption it spanned',()=>{
 assert.deepEqual(overlappingFaults(epoch+500,epoch+8000,windows),['restart']);
 assert.deepEqual(overlappingFaults(epoch+4000,epoch+8000,windows),['restart']);
 assert.deepEqual(overlappingFaults(epoch+6000,epoch+8000,windows),[]);
});
test('open faults and exact boundaries remain auditable',()=>{
 assert.deepEqual(overlappingFaults(epoch,epoch+1000,windows),['restart']);
 assert.deepEqual(overlappingFaults(epoch+6000,epoch+8000,[{id:'open',startedAt:windows[0].startedAt}]),['open']);
});
test('invalid timestamps never silently classify a fault as absent',()=>{
 assert.throws(()=>overlappingFaults(epoch+2,epoch,windows));
 assert.throws(()=>overlappingFaults(epoch,epoch+10,[{id:'bad',startedAt:'invalid'}]));
});
