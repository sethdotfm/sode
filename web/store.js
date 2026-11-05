// Per-browser storage that never throws: localStorage can be missing,
// blocked or full (private windows, cleared site data). Reads give the
// default, writes are dropped. A value of null removes the key.

export const readStr = (k) => { try { return localStorage.getItem(k); } catch (e) { return null; } };
export const writeStr = (k, v) => {
  try {
    if (v == null) localStorage.removeItem(k);
    else localStorage.setItem(k, v);
  } catch (e) {}
};
export const readJSON = (k, d = null) => { try { return JSON.parse(localStorage.getItem(k)) ?? d; } catch (e) { return d; } };
export const writeJSON = (k, v) => writeStr(k, v == null ? null : JSON.stringify(v));
