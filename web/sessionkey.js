// The user key of a Session-mode panel: the page trades its signed-in session for a key over
// the socket and sends it as a Bearer header to /api/v1 (the server ends it with the session).
// Held in memory only: a reload asks again and a sign-out drops it. lock.js owns the PIN pad.
import { on, emit, onMsg, wsSend } from './core.js';
import { sessionStep, sessionKeyReq, keyFrom, renewable, withKey, guardFetch } from './sessionkeyrules.js';

export { jsonCall } from './sessionkeyrules.js';

let key = '';
let access = null;
let started = false;
let prompted = false; // the PIN pad is raised once per sign-out; a cancel is not re-asked
let asking = false; // one request at a time: each answer is a new key, and the session holds at most 16
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
  if (ended) return 'ended';
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

// Every call shares one Retry-After wait; the page's own stop is `ended` below.
const send = guardFetch(fetch);
let worked = ''; // the last key Sode accepted
let ended = null; // the final answer, once a fresh key was refused too

// The first call to see key k refused drops it and asks for a fresh one, while signed in; the rest wait for it.
// Signed out, every call waits for the sign-in instead. Neither waits on the network.
function renew(k) {
  if (key !== k) return;
  setKey('');
  if (!access?.user || access.locked || asking) return;
  asking = true;
  wsSend(sessionKeyReq(++seq, keyName));
}

// fetch with the session key. A refused key is renewed once; a fresh key that is refused before it ever worked
// ends the page's calls for good, so a page never loops on a key Sode keeps ending.
export async function sessionFetch(path, init = {}) {
  for (let tries = 0; ; tries++) {
    if (ended) return ended.clone();
    const k = await keyReady();
    if (ended) return ended.clone();
    const res = await send(path, withKey(init, k));
    if (res.status !== 401) {
      if (res.ok) worked = k;
      return res;
    }
    const code = (
      await res
        .clone()
        .json()
        .catch(() => ({}))
    )?.error?.code;
    if (!renewable(res.status, code)) return res;
    if (tries > 0 && (worked !== k || tries > 2)) {
      ended = res.clone();
      for (const fn of watchers) fn('', 'ended');
      return res;
    }
    renew(k);
  }
}
