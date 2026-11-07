// Per-instance branding: logo text and optional border colour. Display only; the red lost-link outline (.sode__frame) stays on top.
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
  // The server draws the tab and home-screen icons in this colour (icon.go).
  const tab = document.querySelector('link[rel="icon"]');
  if (tab) tab.href = iconURL(192, color);
  const touch = document.querySelector('link[rel="apple-touch-icon"]');
  if (touch) touch.href = iconURL(180, color);
  emit('brand', { text, color });
});
