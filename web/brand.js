// Brand: logo text and optional border color. Display only; the red lost-link outline (.sode__frame) stays on top.
// The accent is made from the border color (web/accent.js, docs/color-roles.md); with no border color it is patchbay's.
import { on, emit, S } from './core.js';
import { iconURL, setBrand } from './brandicon.js';
import { accentFor, accentTokens, brandHex, ACCENT_TOKENS } from './accent.js';

// prepaint.js reads it before first paint, so a reload shows no periwinkle before the hello.
const ACCENT_KEY = 'sode-accent';

const frame = document.createElement('div');
frame.className = 'sode__brandframe';
frame.setAttribute('aria-hidden', 'true');
frame.hidden = true;
document.body.append(frame);

on('helloCfg', (cfg) => {
  const tokens = showBrand(cfg.brandText, cfg.brandBorder);
  try {
    if (tokens) localStorage.setItem(ACCENT_KEY, JSON.stringify(tokens));
    else localStorage.removeItem(ACCENT_KEY);
  } catch {}
});

// Sets the --pb-accent* tokens on the root from the border color, or removes them. Returns the tokens set, or null.
function showAccent(color) {
  const root = document.documentElement.style;
  const hex = brandHex(color, S.brandColors);
  const tokens = hex ? accentTokens(accentFor(hex)) : null;
  for (const k of ACCENT_TOKENS) {
    if (tokens) root.setProperty(k, tokens[k]);
    else root.removeProperty(k);
  }
  return tokens;
}

// The setup wizard calls it on each keystroke, before any save.
export function showBrand(brandText, brandBorder) {
  const text = (brandText || '').trim() || 'Sode';
  for (const m of document.querySelectorAll('.sode__mark, .sode-m__mark')) m.textContent = text;
  const color = brandBorder || '';
  setBrand(frame, color);
  frame.hidden = !color;
  const tokens = showAccent(color);
  // The server draws the tab and home-screen icons in this color (icon.go).
  const tab = document.querySelector('link[rel="icon"]');
  if (tab) tab.href = iconURL(192, color);
  const touch = document.querySelector('link[rel="apple-touch-icon"]');
  if (touch) touch.href = iconURL(180, color);
  emit('brand', { text, color });
  return tokens;
}
