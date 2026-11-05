// Runs in <head> before first paint (a classic script, not a module, so it
// blocks): applies the theme, and on the main page the layout and the flip,
// so there is no flash. Each is this browser's own choice, else the global
// default last cached from the server (as resolve() in prefs.js does).
// device.js keeps them up to date afterwards. The other pages load it with
// data-only="theme" and get the theme alone.
(() => {
  const root = document.documentElement;
  const themeOnly = document.currentScript?.dataset.only === 'theme';
  let g = {};
  const get = (k) => { try { return localStorage.getItem(k); } catch (e) { return null; } };
  try { g = JSON.parse(get('sode-device-defaults')) || {}; } catch (e) {}
  const pick = (key, name, ok, def) => {
    const own = get(key);
    if (ok.includes(own)) return own;
    const v = typeof g[name] === 'boolean' ? (g[name] ? 'on' : 'off') : g[name];
    return ok.includes(v) ? v : def;
  };
  const theme = pick('sode-theme', 'theme', ['system', 'light', 'dark'], 'system');
  if (theme !== 'system') root.dataset.theme = theme;
  if (themeOnly) return;
  let layout = pick('sode-layout', 'layout', ['auto', 'phone', 'desktop'], 'auto');
  if (layout === 'auto') layout = matchMedia('(max-width: 700px) and (pointer: coarse)').matches ? 'phone' : 'desktop';
  root.dataset.layout = layout;
  if (layout === 'phone') root.dataset.pbLayout = 'phone';
  if (pick('sode-flip', 'flip', ['on', 'off'], 'off') === 'on') root.dataset.flip = '';
})();
