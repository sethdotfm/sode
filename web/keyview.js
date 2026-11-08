// DOM pieces shared by the key lists: one key's row, the one-time key display with Copy and a QR code.
// The rules are in keyrules.js and the QR encoder in qr.js; nothing here is stored.

import { el, icon } from './core.js';
import { keyFacts } from './keyrules.js';
import { qrMatrix, qrPath } from './qr.js';

const SVG = 'http://www.w3.org/2000/svg';

// Black on white in both themes, set as attributes and not tokens: a camera needs the contrast,
// and the quiet zone must stay light under a dark page.
export function qrNode(text) {
  let m;
  try {
    m = qrMatrix(text);
  } catch {
    return null;
  }
  const { d, box } = qrPath(m);
  const svg = document.createElementNS(SVG, 'svg');
  svg.setAttribute('viewBox', `0 0 ${box} ${box}`);
  svg.setAttribute('class', 'sode__qr');
  svg.setAttribute('role', 'img');
  svg.setAttribute('aria-label', 'QR code of the key');
  svg.setAttribute('shape-rendering', 'crispEdges');
  const bg = document.createElementNS(SVG, 'rect');
  for (const [k, v] of [
    ['width', box],
    ['height', box],
    ['fill', '#ffffff'],
  ])
    bg.setAttribute(k, v);
  const path = document.createElementNS(SVG, 'path');
  path.setAttribute('d', d);
  path.setAttribute('fill', '#000000');
  svg.append(bg, path);
  return svg;
}

async function copyText(text, field) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    field.select();
    try {
      return document.execCommand('copy');
    } catch {
      return false;
    }
  }
}

// The key, shown once. Nothing keeps it: drop the node and it is gone.
export function secretNode(secret) {
  const field = el('input', 'pb-input sode__secret-key');
  field.readOnly = true;
  field.value = secret;
  field.spellcheck = false;
  field.setAttribute('aria-label', 'API key');
  field.addEventListener('focus', () => field.select());
  const say = el('span', 'sode__hint');
  say.setAttribute('role', 'status');
  const copy = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('copy'), 'Copy');
  copy.type = 'button';
  copy.addEventListener('click', async () => {
    say.textContent = (await copyText(secret, field)) ? 'Copied.' : 'Copy failed. Select the key and copy it by hand.';
  });
  const qr = qrNode(secret);
  return el(
    'div',
    'sode__secret',
    el('p', 'sode__hint', 'Copy this key now. Sode shows it once and keeps only a hash of it.'),
    el('div', 'sode__secret-row', field, copy, say),
    ...(qr ? [qr] : []),
  );
}

// One key. onRevoke is armed by a first press, as the Remove button of a user is.
export function keyRow(k, now, onRevoke) {
  const f = keyFacts(k, now);
  const rm = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Revoke');
  rm.type = 'button';
  const disarm = () => {
    rm.dataset.armed = '';
    rm.textContent = 'Revoke';
    rm.classList.replace('pb-btn--danger', 'pb-btn--ghost');
  };
  rm.addEventListener('click', () => {
    if (!rm.dataset.armed) {
      rm.dataset.armed = '1';
      rm.textContent = 'Confirm revoke';
      rm.classList.replace('pb-btn--ghost', 'pb-btn--danger');
      return;
    }
    rm.disabled = true;
    onRevoke(k);
  });
  rm.addEventListener('blur', disarm);
  const facts = `Created ${f.created}, ${f.used}, ${f.expires}.`;
  return el(
    'div',
    'sode__key-row',
    el('div', 'sode__key-main', el('strong', null, f.name), el('span', f.expired ? 'sode__hint sode__error-text' : 'sode__hint', facts)),
    rm,
  );
}
