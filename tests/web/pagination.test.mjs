import test from 'node:test';
import assert from 'node:assert/strict';
import {collectPages, collectionURL} from '../../web/src/lib/api/pagination.ts';

test('configuration collections retain filters and page size through every cursor', async () => {
  const urls = [], cursor = 'second+/=?中文';
  const items = await collectPages('/positions?q=name&limit=50', async url => {
    urls.push(url);
    return urls.length === 1 ? {items: [{id: 'first'}], nextCursor: cursor} : {items: [{id: 'last'}], nextCursor: null};
  });
  assert.deepEqual(items.map(i => i.id), ['first', 'last']);
  const query = new URLSearchParams(urls[1].split('?')[1]);
  assert.equal(query.get('cursor'), cursor);
  assert.deepEqual(query.getAll('limit'), ['50']);
  assert.equal(query.get('q'), 'name');
  assert.equal(collectionURL('/positions?', '', 50), '/positions?limit=50');
});

test('later-page errors and cursor cycles prevent partial data from reaching mutations', async () => {
  let calls = 0;
  await assert.rejects(collectPages('/identities', async () => {
    if (++calls === 2) throw Error('unavailable');
    return {items: ['partial'], nextCursor: 'next'};
  }), /unavailable/);
  calls = 0;
  await assert.rejects(collectPages('/identities', async () => {
    calls++;
    return {items: ['partial'], nextCursor: 'cycle'};
  }), /Repeated collection cursor/);
  assert.equal(calls, 2);
});
