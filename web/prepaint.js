// Runs in <head> before first paint (a classic script, not a module, so it
// blocks): applies the theme, and on the main page the layout and the flip,
// so there is no flash. Each is this browser's own choice, else the logged-in
// user's cached default, else the global default last cached from the server (as resolve() in prefs.js does).
// device.js keeps them up to date afterwards. The other pages load it with
// data-only="theme" and get the theme alone.
(() => {
  const root = document.documentElement;
  const themeOnly = document.currentScript?.dataset.only === 'theme';
  let g = {};
  const get = (k) => { try { return localStorage.getItem(k); } catch (e) { return null; } };
  try { g = JSON.parse(get('sode-device-defaults')) || {}; } catch (e) {}
  let mine = {};
  try {
    // Only with a session token kept (else this load is a guest): as cachedProfile() in prefs.js.
    let tab = null;
    try { tab = sessionStorage.getItem('sode-tab-session'); } catch (e) {}
    const c = JSON.parse(get('sode-profile-prefs'));
    if ((get('sode-session') || tab) && c && typeof c.user === 'string' && c.user && c.prefs && typeof c.prefs === 'object') mine = c.prefs;
  } catch (e) {}
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
  // TEMPORARY design switch: ?sections=panels|rules|bands (empty clears), kept in sode-sections.
  try {
    const q = new URLSearchParams(location.search).get('sections');
    if (q !== null) localStorage.setItem('sode-sections', q);
  } catch (e) {}
  const sec = get('sode-sections');
  if (['panels', 'rules', 'bands'].includes(sec)) root.dataset.sections = sec;
  let layout = pick('sode-layout', 'layout', ['auto', 'phone', 'desktop'], 'auto');
  if (layout === 'auto') layout = matchMedia('(max-width: 700px) and (pointer: coarse)').matches ? 'phone' : 'desktop';
  root.dataset.layout = layout;
  if (layout === 'phone') root.dataset.pbLayout = 'phone';
  if (pick('sode-flip', 'flip', ['on', 'off'], 'off') === 'on') root.dataset.flip = '';
})();
