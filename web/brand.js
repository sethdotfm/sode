// Per-instance branding (issue 126): the logo text replaces "Sode" in the
// header and page title, and an optional colour border goes round the window.
// Display only; the red lost-link outline (.sode__frame) stays on top.
import { on } from './core.js';

const frame = document.createElement('div');
frame.className = 'sode__brandframe';
frame.setAttribute('aria-hidden', 'true');
frame.hidden = true;
document.body.append(frame);

on('helloCfg', (cfg) => {
  const text = (cfg.brandText || '').trim() || 'Sode';
  for (const m of document.querySelectorAll('.sode__mark, .sode-m__mark')) m.textContent = text;
  const logTitle = document.querySelector('.sode-log__title');
  if (logTitle) logTitle.textContent = `${text} Communication Log`;
  document.title = logTitle ? `${text} Communication Log \u00b7 ${text}` : text;
  const color = cfg.brandBorder || '';
  if (color) frame.dataset.brand = color;
  else delete frame.dataset.brand;
  frame.hidden = !color;
});
