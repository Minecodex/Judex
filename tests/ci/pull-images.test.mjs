import test from 'node:test';
import assert from 'node:assert/strict';
import {pullImages} from '../../scripts/ci/pull-images.mjs';

const badGateway = {status: 1, stderr: 'received unexpected HTTP status: 502 Bad Gateway'};
const success = {status: 0};

test('a transient registry failure retries the same image before pulling the next', async () => {
  const calls = [], delays = [], retries = [];
  await pullImages(['postgres:17-alpine', 'chrislusf/seaweedfs:4.47'], {
    pull: image => { calls.push(image); return calls.length === 1 ? badGateway : success; },
    wait: async ms => { delays.push(ms); },
    onRetry: event => retries.push(event),
  });
  assert.deepEqual(calls, ['postgres:17-alpine', 'postgres:17-alpine', 'chrislusf/seaweedfs:4.47']);
  assert.deepEqual(delays, [2000]);
  assert.deepEqual(retries, [{image: 'postgres:17-alpine', attempt: 1, delayMs: 2000}]);
});

test('network timeouts have the same bounded recovery path', async () => {
  const delays = [];
  let count = 0;
  await pullImages(['postgres:17-alpine'], {
    pull: () => ++count === 1 ? {status: null, error: {code: 'ETIMEDOUT', message: 'timed out'}} : success,
    wait: async ms => { delays.push(ms); }, onRetry: () => {},
  });
  assert.equal(count, 2);
  assert.deepEqual(delays, [2000]);
});

test('permanent pull errors fail immediately and do not continue to another image', async () => {
  for (const stderr of ['pull access denied', 'manifest unknown', 'digest verification failed', 'unknown error']) {
    const calls = [];
    await assert.rejects(pullImages(['postgres:17-alpine', 'chrislusf/seaweedfs:4.47'], {
      pull: image => { calls.push(image); return {status: 1, stderr}; },
      wait: async () => assert.fail('permanent errors must not be retried'),
      onRetry: () => assert.fail('permanent errors must not be retried'),
    }), /Docker pull failed.*\(1\/3\)/);
    assert.deepEqual(calls, ['postgres:17-alpine']);
  }
});

test('exhausting transient retries fails the gate without pulling later images', async () => {
  const calls = [], delays = [];
  await assert.rejects(pullImages(['postgres:17-alpine', 'chrislusf/seaweedfs:4.47'], {
    pull: image => { calls.push(image); return badGateway; },
    wait: async ms => { delays.push(ms); }, onRetry: () => {},
  }), /Docker pull failed.*\(3\/3\)/);
  assert.deepEqual(calls, Array(3).fill('postgres:17-alpine'));
  assert.deepEqual(delays, [2000, 4000]);
});
