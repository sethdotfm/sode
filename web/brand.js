// Per-instance branding (issue 126): the logo text replaces "Sode" in the
// header, and an optional colour border goes round the window (and colours
// the tab and app icons). Pages set their own titles from the `brand` event.
// Display only; the red lost-link outline (.sode__frame) stays on top.
import { on, emit } from './core.js';
import { iconURL, setBrand } from './brandicon.js';

const frame = document.createElement('div');
frame.className = 'sode__brandframe';
frame.setAttribute('aria-hidden', 'true');
frame.hidden = true;
document.body.append(frame);

on('helloCfg', (cfg) => {
  const text = (cfg.brandText || '').trim() || 'Sode';
  for (const m of document.querySelectorAll('.sode__mark, .sode-m__mark')) m.textContent = text;
  const color = cfg.brandBorder || '';
  setBrand(frame, color);
  frame.hidden = !color;
  // The tab icon and the home-screen icon take the colour too (the server
  // draws them, see icon.go); the manifest follows the config on its own.
  const tab = document.querySelector('link[rel="icon"]');
  if (tab) tab.href = iconURL(192, color);
  const touch = document.querySelector('link[rel="apple-touch-icon"]');
  if (touch) touch.href = iconURL(180, color);
  emit('brand', { text, color });
});
