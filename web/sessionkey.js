// The user key of a Session-mode panel: the page trades its signed-in session for a key over
// the socket and sends it as a Bearer header to /api/v1 (the server ends it with the session).
// Held in memory only: a reload asks again and a sign-out drops it. lock.js owns the PIN pad.
import { on, emit, onMsg, wsSend } from './core.js';
import { sessionStep, sessionKeyReq, keyFrom, renewable, withKey } from './sessionkeyrules.js';

export { jsonCall } from './sessionkeyrules.js';

const RENEW_MS = 5000;
let key = '';
let access = null;
let started = false;
let prompted = false; // the PIN pad is raised once per sign-out; a cancel is not re-asked
let asking = false; // each request replaces the earlier key, so a second one would strand the answer to the first
let seq = 0;
let keyName = '';
let waiting = [];
const watchers = [];

export const sessionKey = () => key;
export const onSessionKey = (fn) => watchers.push(fn);
// Names the key this page asks for, before startSession.
export const nameSessionKey = (name) => {
  keyName = name;
};

function setKey(k) {
  if (k === key) return;
  key = k;
  if (k) {
    for (const r of waiting) r(k);
    waiting = [];
  }
  for (const fn of watchers) fn(k);
}

function drive() {
  if (!started || !access) return 'idle';
  const step = sessionStep(access, key);
  if (step === 'sign-in' && !prompted) {
    prompted = true;
    emit('login');
  } else if (step === 'request' && !asking) {
    asking = true;
    wsSend(sessionKeyReq(++seq, keyName));
  }
  return step;
}

// Call once the page knows the panel runs as the signed-in user.
export function startSession() {
  started = true;
  drive();
}

// Raises the PIN pad again after a cancel.
export function signInAgain() {
  prompted = false;
  drive();
}

on('access', (a) => {
  access = a;
  if (!a.user || a.locked) setKey('');
  if (a.user) prompted = false;
  const step = drive();
  for (const fn of watchers) fn(key, step);
});

onMsg('sessionKeyResult', (m) => {
  asking = false;
  setKey(keyFrom(m));
});
on('wsclose', () => {
  asking = false;
});

const keyReady = () => (key ? Promise.resolve(key) : new Promise((r) => waiting.push(r)));

// fetch with the session key. A key the server ended is asked for again once, while still signed in.
export async function sessionFetch(path, init = {}) {
  const res = await fetch(path, withKey(init, await keyReady()));
  if (res.status !== 401) return res;
  const code = (
    await res
      .clone()
      .json()
      .catch(() => ({}))
  )?.error?.code;
  if (!renewable(res.status, code)) return res;
  const old = key;
  setKey('');
  asking = true;
  wsSend(sessionKeyReq(++seq, keyName));
  const fresh = await Promise.race([keyReady(), new Promise((r) => setTimeout(r, RENEW_MS, ''))]);
  if (!fresh || fresh === old) return res;
  return fetch(path, withKey(init, fresh));
}
