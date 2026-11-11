import test from 'node:test';
import assert from 'node:assert/strict';
import { isLoopback, parseShareHost, shareBase } from '../sharehostrules.js';

test('loopback names are the ones a phone cannot reach', () => {
  for (const h of ['localhost', '127.0.0.1', '::1', '[::1]', 'sode.localhost']) assert.equal(isLoopback(h), true, h);
  for (const h of ['192.168.1.20', 'sode.lan', 'example.com', '']) assert.equal(isLoopback(h), false, h);
});

test('a typed address needs a host that is not loopback, and borrows the page scheme', () => {
  assert.equal(parseShareHost('192.168.1.20:8080', 'http:').origin, 'http://192.168.1.20:8080');
  assert.equal(parseShareHost('https://sode.lan', 'http:').origin, 'https://sode.lan');
  for (const bad of ['', 'localhost:8080', 'ftp://x', '///']) assert.equal(parseShareHost(bad), null, bad);
});

test('only a loopback page has its address swapped', () => {
  assert.equal(shareBase('http://localhost:8081/panels/editor', '192.168.1.20:8081'), 'http://192.168.1.20:8081/panels/editor');
  assert.equal(shareBase('http://localhost:8081/panels/editor', ''), 'http://localhost:8081/panels/editor');
  assert.equal(shareBase('http://localhost:8081/x', 'nonsense host'), 'http://localhost:8081/x');
  assert.equal(shareBase('http://sode.lan/x', '192.168.1.20'), 'http://sode.lan/x');
});
