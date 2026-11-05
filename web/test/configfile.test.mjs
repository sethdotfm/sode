import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileName, fileProblem, sizeText, MAX_BYTES } from '../configfilerules.js';

test('file name: date, site, safe characters', () => {
  const now = new Date(2026, 10, 4, 12);
  assert.equal(fileName({ now, site: 'Studio B' }), '2026-11-04-Sode-Server-Config-Studio-B.yaml');
  assert.equal(fileName({ now, site: '../évil/ name?' }), '2026-11-04-Sode-Server-Config-evil-name.yaml');
  assert.equal(fileName({ now, site: '' }), '2026-11-04-Sode-Server-Config.yaml');
});

test('a picked file', () => {
  assert.ok(fileProblem(0, ''));
  assert.ok(fileProblem(5, '  \n'));
  assert.ok(fileProblem(MAX_BYTES + 1, 'x'));
  assert.equal(fileProblem(10, 'router: x'), '');
  assert.equal(sizeText(12), '12 bytes');
  assert.equal(sizeText(4096), '4 KB');
});

test('the limit matches the server', () => {
  const go = readFileSync(new URL('../../internal/app/configfile.go', import.meta.url), 'utf8');
  assert.match(go, /const maxConfigFile = 2 << 20/);
  assert.equal(MAX_BYTES, 2 << 20);
});
