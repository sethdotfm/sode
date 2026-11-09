// A panel's link as a read-only field with a Copy link button, for the editor's Identity section and the Share
// modal. The text comes from linkFor only. A panel with its own API client has no link without its key.
import { el, icon } from './core.js';
import { linkFor } from './panellink.js';

const COPIED_MS = 2000;

// Copies text, and says false when the browser refused (no secure context, no permission).
export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

export function linkRow(panel) {
  const box = el('div', 'sode__field sode-linkrow');
  if (panel.acts === 'api') {
    box.append(el('p', 'sode__hint', 'This panel acts as its own API client, so its link needs the key. Use Link with key in Share.'));
    return box;
  }
  const link = linkFor(panel);
  const input = el('input', 'pb-input sode-linkrow__text');
  input.readOnly = true;
  input.spellcheck = false;
  input.value = link;
  input.setAttribute('aria-label', 'Panel link');
  input.addEventListener('focus', () => input.select());
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const copy = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('copy'), 'Copy link');
  copy.type = 'button';
  copy.addEventListener('click', async () => {
    if (await copyText(link)) {
      note.textContent = 'Copied.';
      setTimeout(() => (note.textContent = ''), COPIED_MS);
    } else {
      input.select();
      note.textContent = 'Copy failed. The link is selected: copy it by hand.';
    }
  });
  box.append(el('span', 'sode__field-label', 'Link'), el('div', 'sode-linkrow__row', input, copy), note);
  return box;
}
