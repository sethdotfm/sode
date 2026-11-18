// The preset editor's Item link: the link, Copy link and its QR code. Link rules are in itemlink.js.
import { el, S } from './core.js';
import { labeled } from './widgets.js';
import { qrNode } from './keyview.js';
import { linkBase } from './sharehost.js';
import { presetLink } from './itemlink.js';

const COPIED_MS = 2000;

export function itemLinkBox() {
  const input = el('input', 'pb-input sode__code');
  input.readOnly = true;
  input.spellcheck = false;
  input.setAttribute('aria-label', 'Item link');
  input.addEventListener('focus', () => input.select());
  const status = el('span', 'pb-status');
  status.setAttribute('role', 'status');
  const copy = el('button', 'pb-btn pb-btn--outline', 'Copy link');
  copy.type = 'button';
  const qrBox = el('div', 'sode__itemlink-qr');
  const box = el('div', 'sode__itemlink', el('div', 'sode__itemlink-row', labeled('Item link', input), copy, status), qrBox);
  box.hidden = true;
  let timer = 0;
  const copiedOff = () => {
    clearTimeout(timer);
    copy.classList.replace('pb-btn--ok', 'pb-btn--outline');
    copy.textContent = 'Copy link';
    copy.style.minWidth = '';
  };
  copy.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(input.value);
    } catch {
      input.select();
      copiedOff();
      return status.replaceChildren('The link is selected: copy it by hand.');
    }
    status.replaceChildren();
    clearTimeout(timer);
    copy.style.minWidth = `${copy.offsetWidth}px`; // keeps Copy link's width
    copy.classList.replace('pb-btn--outline', 'pb-btn--ok');
    copy.textContent = 'Copied!';
    timer = setTimeout(copiedOff, COPIED_MS);
  });
  return {
    box,
    // A preset of this install; This browser presets have no Item link.
    showPreset(id) {
      input.value = presetLink(linkBase(), id, S.installUid);
      const qr = qrNode(input.value, 'QR code of the Item link');
      qrBox.replaceChildren(qr || el('p', 'sode__hint', 'This link is too long for a QR code. Copy it instead.'));
      status.replaceChildren();
      copiedOff();
      box.hidden = false;
      input.select();
    },
  };
}
