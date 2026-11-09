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

// The two answers for a key whose session ended: asked again once while still signed in.
export const renewable = (status, code) => status === 401 && (code === 'invalid_key' || code === 'key_expired');

export const withKey = (init, key) => ({ ...init, headers: { ...init.headers, Authorization: `Bearer ${key}` } });
