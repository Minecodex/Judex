import test from 'node:test';
import assert from 'node:assert/strict';
import {assessContextWindow} from './kimi-context-preflight.mjs';
test('rejects a shrunken window before paid calls when native retained history cannot fit',()=>{
 const measurements=[{tokensAfter:44759},{tokensAfter:45299},{tokensAfter:44824}];
 const result=assessContextWindow(measurements,32768,16384);
 assert.equal(result.admitted,false);assert.equal(result.observedMinimum,44759);
 assert.equal(assessContextWindow(measurements,65536,16384).admitted,true);
});
test('ignores invalid measurements and uses recent native context metrics',()=>{
 assert.equal(assessContextWindow([{tokensAfter:-1},{tokensAfter:null}],32768,8192).observations,0);
 const result=assessContextWindow([{tokensAfter:100},{tokensAfter:50000},{tokensAfter:51000},{tokensAfter:52000}],65536,16384);
 assert.equal(result.observedMinimum,50000);assert.equal(result.admitted,false);
});
