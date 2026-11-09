// Rules for a Session-mode panel's user key (sessionkey.js). Pure, so tests load it.

// What a Session-mode panel does next, from the lock access state and the key it holds.
// A page lock draws its own pad, so the panel waits behind it.
export function sessionStep(access, key) {
  if (access.locked) return 'wait';
  if (access.user) return key ? 'ready' : 'request';
  if (access.enforced) return 'wait';
  return (access.lens?.length || 0) > 0 ? 'sign-in' : 'no-users';
}

// keyName shows in History as "Ana (Panel p-1a2b3c4d)"; left out, the server names it Session.
export const sessionKeyReq = (id, keyName = '') => ({ t: 'sessionKey', id, ...(keyName ? { keyName } : {}) });

export const keyFrom = (m) => (m.ok && m.key ? m.key : '');

// Sode's answer for a key that worked once and ended. Final: sending the key again cannot help.
export const endedCode = (status, code) => status === 401 && (code === 'key_revoked' || code === 'key_expired');

// The answers a session key is asked again for, once, while still signed in. invalid_key is an older Sode's word.
export const renewable = (status, code) => endedCode(status, code) || (status === 401 && code === 'invalid_key');

// Retry-After in seconds as ms; 0 for none.
export function retryAfterMs(v) {
  const s = Number(v);
  return v != null && Number.isFinite(s) && s > 0 ? s * 1000 : 0;
}

const errorCode = (res) =>
  res
    .clone()
    .json()
    .then((b) => b?.error?.code)
    .catch(() => undefined);

const pause = (ms) => new Promise((r) => setTimeout(r, ms));

// Wraps a fetcher so a page stops asking when Sode says to. After a 429 with Retry-After every call waits it
// out first (the address may be in the failed-key lockout). After an answer final(status, code) picks, every
// call gets that answer again without the network, so no poll, keepalive or stream retry keeps going.
export function guardFetch(fetcher, { final = () => false, now = Date.now, sleep = pause } = {}) {
  let until = 0;
  let last = null;
  return async (path, init) => {
    for (let w = until - now(); w > 0 && !last; w = until - now()) await sleep(w);
    if (last) return last.clone();
    const res = await fetcher(path, init);
    if (res.status === 429) {
      const ms = retryAfterMs(res.headers?.get('Retry-After'));
      if (ms) until = Math.max(until, now() + ms);
    } else if (res.status === 401 && final(res.status, await errorCode(res))) last = res.clone();
    return res;
  };
}

export const withKey = (init, key) => ({ ...init, headers: { ...init.headers, Authorization: `Bearer ${key}` } });

// A JSON call over fetcher (sessionFetch, or fetch with a URL key): the body, null for 204, else an Error
// carrying status, code and the server's message. failed(status) words an answer with no message, down a
// dead network (status 0); without down the fetch error passes through. Callers keep their own texts.
export function jsonCall(fetcher, { failed = () => '', down } = {}) {
  return async (path, init) => {
    let res;
    try {
      res = await fetcher(path, init);
    } catch (e) {
      if (down === undefined) throw e;
      throw Object.assign(new Error(down), { status: 0 });
    }
    if (res.status === 204) return null;
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw Object.assign(new Error(body.error?.message || failed(res.status)), { status: res.status, code: body.error?.code });
    return body;
  };
}
