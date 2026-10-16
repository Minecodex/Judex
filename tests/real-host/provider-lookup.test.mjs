import test from 'node:test';
import assert from 'node:assert/strict';
import {providerLookup} from './provider-lookup.mjs';

test('test pin supports both Node lookup result forms',()=>{
 const lookup=providerLookup('provider.test','104.26.10.94',()=>assert.fail('Unexpected DNS lookup'));
 lookup('provider.test',{all:true},(error,result)=>{assert.equal(error,null);assert.deepEqual(result,[{address:'104.26.10.94',family:4}]);});
 lookup('provider.test',{},(error,address,family)=>{assert.equal(error,null);assert.equal(address,'104.26.10.94');assert.equal(family,4);});
});
test('test pin does not redirect other hosts',()=>{
 let actual;
 providerLookup('provider.test','104.26.10.94',(host,options,callback)=>{actual=host;callback(null,'127.0.0.1',4);})('other.test',{},()=>{});
 assert.equal(actual,'other.test');
});
test('hostnames cannot be passed as pin addresses',()=>{
 assert.throws(()=>providerLookup('provider.test','another.test'),/literal IPv4/);
});
