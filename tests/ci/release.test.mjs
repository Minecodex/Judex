import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {sha256, verifyRelease} from '../../scripts/release/artifacts.mjs';

function candidate(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'judex-release-'));
  t.after(() => fs.rmSync(directory, {recursive: true, force: true}));
  fs.writeFileSync(path.join(directory, 'server'), 'immutable binary bytes');
  fs.writeFileSync(path.join(directory, 'manifest.json'), JSON.stringify({version: '0.0.1-beta', artifacts: ['server']}));
  fs.writeFileSync(path.join(directory, 'checksums.txt'), ['manifest.json', 'server'].map(name => sha256(fs.readFileSync(path.join(directory, name))) + '  ' + name).join('\n') + '\n');
  return directory;
}

test('publication accepts the complete candidate and refuses changed bytes', t => {
  const directory = candidate(t);
  assert.equal(verifyRelease(directory).version, '0.0.1-beta');
  fs.writeFileSync(path.join(directory, 'server'), 'different binary bytes');
  assert.throws(() => verifyRelease(directory), /Checksum mismatch: server/);
});

test('publication refuses incomplete checksum coverage', t => {
  const directory = candidate(t);
  const sums = fs.readFileSync(path.join(directory, 'checksums.txt'), 'utf8');
  fs.writeFileSync(path.join(directory, 'checksums.txt'), sums.split('\n')[0] + '\n');
  assert.throws(() => verifyRelease(directory), /Incomplete release checksums/);
});

test('publication refuses duplicate entries and paths outside the release', t => {
  const directory = candidate(t);
  const sums = fs.readFileSync(path.join(directory, 'checksums.txt'), 'utf8');
  for (const suffix of [sums.split('\n')[0], sha256(Buffer.from('x')) + '  ../outside']) {
    fs.writeFileSync(path.join(directory, 'checksums.txt'), sums + suffix + '\n');
    assert.throws(() => verifyRelease(directory), /Invalid release checksum entry/);
  }
});

test('publication refuses symlink substitution even when the content matches', t => {
  const directory = candidate(t);
  fs.renameSync(path.join(directory, 'server'), path.join(directory, 'outside'));
  fs.symlinkSync('outside', path.join(directory, 'server'));
  assert.throws(() => verifyRelease(directory), /Checksum mismatch: server/);
});
