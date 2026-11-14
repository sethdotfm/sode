// Classic script in <head> (blocks) so there is no flash: theme, plus layout, flip and compact on the main page (other pages: data-only="theme").
// Repeats prefs.js resolve() and cachedProfile() inline; web/test/prepaint.test.mjs fails if they disagree.
(() => {
  const root = document.documentElement;
  const themeOnly = document.currentScript?.dataset.only === 'theme';
  let g = {};
  const get = (k) => {
    try {
      return localStorage.getItem(k);
    } catch {
      return null;
    }
  };
  try {
    g = JSON.parse(get('sode-device-defaults')) || {};
  } catch {}
  let mine = {};
  try {
    // Only with a session token kept (else this load is a guest): as cachedProfile() in prefs.js.
    let tab = null;
    try {
      tab = sessionStorage.getItem('sode-tab-session');
    } catch {}
    const c = JSON.parse(get('sode-profile-prefs'));
    if ((get('sode-session') || tab) && c && typeof c.user === 'string' && c.user && c.prefs && typeof c.prefs === 'object') mine = c.prefs;
  } catch {}
  const word = (v) => (typeof v === 'boolean' ? (v ? 'on' : 'off') : v);
  const pick = (key, name, ok, def) => {
    const own = get(key);
    if (ok.includes(own)) return own;
    for (const v of [word(mine[name]), word(g[name])]) if (ok.includes(v)) return v;
    return def;
  };
  const theme = pick('sode-theme', 'theme', ['system', 'light', 'dark'], 'system');
  if (theme !== 'system') root.dataset.theme = theme;
  if (themeOnly) return;
  let layout = pick('sode-layout', 'layout', ['auto', 'phone', 'desktop'], 'auto');
  if (layout === 'auto') layout = matchMedia('(max-width: 700px) and (pointer: coarse)').matches ? 'phone' : 'desktop';
  root.dataset.layout = layout;
  if (layout === 'phone') root.dataset.pbLayout = 'phone';
  if (pick('sode-flip', 'flip', ['on', 'off'], 'off') === 'on') root.dataset.flip = '';
  if (pick('sode-compact', 'compact', ['on', 'off'], 'off') === 'on') root.dataset.compact = '';
})();
