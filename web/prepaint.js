// Classic script in <head> (blocks) so there is no flash: theme, plus layout, flip and compact on the main page (other pages: data-only="theme").
// Repeats prefs.js resolve() and cachedProfile() inline; web/test/prepaint.test.mjs fails if they disagree.
(() => {
  const root = document.documentElement;
  const themeOnly = document.currentScript?.dataset.only === 'theme';
  // v2 reads no v1 keys: the first v2 load clears every sode-* key, then stamps. First on every
  // page, so nothing reads a v1 key before it goes. The stamp rises with config.Format only when old keys would mislead.
  try {
    if (localStorage.getItem('sode-format') !== '2') {
      for (let i = localStorage.length - 1; i >= 0; i--) {
        const k = localStorage.key(i);
        if (k?.startsWith('sode-')) localStorage.removeItem(k);
      }
      localStorage.setItem('sode-format', '2');
    }
  } catch {}
  let g = {};
  const get = (k) => {
    try {
      return localStorage.getItem(k);
    } catch {
      return null;
    }
  };
  try {
    g = JSON.parse(get('sode-browser-defaults')) || {};
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
  // The accent from the last page border color, as brand.js left it (ACCENT_TOKENS in accent.js); the hello corrects it.
  try {
    const a = JSON.parse(get('sode-accent'));
    for (const k of ['--pb-accent', '--pb-accent-hover', '--pb-accent-wash']) {
      const v = a?.[k];
      if (typeof v === 'string' && /^light-dark\([#0-9a-f(),. rgba]+\)$/.test(v)) root.style.setProperty(k, v);
    }
  } catch {}
  if (themeOnly) return;
  let layout = pick('sode-layout', 'layout', ['auto', 'phone', 'desktop'], 'auto');
  if (layout === 'auto') layout = matchMedia('(max-width: 700px) and (pointer: coarse)').matches ? 'phone' : 'desktop';
  root.dataset.layout = layout;
  if (layout === 'phone') root.dataset.pbLayout = 'phone';
  if (pick('sode-flip', 'flip', ['on', 'off'], 'off') === 'on') root.dataset.flip = '';
  if (pick('sode-compact', 'compact', ['on', 'off'], 'off') === 'on') root.dataset.compact = '';
})();
