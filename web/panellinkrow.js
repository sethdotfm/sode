// A panel's link as a read-only field with a Copy link button, for the editor's Identity section and the Share
// modal. The text comes from linkFor only. A panel that only acts as its API client has no link without its key; a
// Both panel's plain link opens in Session mode, with the keyed links under Share.
import { el, icon } from './core.js';
import { linkFor } from './panellink.js';
import { linkBase, shareHostField } from './sharehost.js';

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
  let link = linkFor(panel, { base: linkBase() });
  const input = el('input', 'pb-input sode-linkrow__text');
  input.readOnly = true;
  input.spellcheck = false;
  input.value = link;
  input.setAttribute('aria-label', 'Panel link');
  input.addEventListener('focus', () => input.select());
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  if (panel.acts === 'both') note.textContent = 'This link asks for a sign-in. A link with a key, which opens without one, is under Link with key in Share.';
  const copy = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('copy'), 'Copy link');
  copy.type = 'button';
  const rest = note.textContent;
  copy.addEventListener('click', async () => {
    if (await copyText(link)) {
      note.textContent = 'Copied.';
      setTimeout(() => (note.textContent = rest), COPIED_MS);
    } else {
      input.select();
      note.textContent = 'Copy failed. The link is selected: copy it by hand.';
    }
  });
  const host = shareHostField(() => {
    link = linkFor(panel, { base: linkBase() });
    input.value = link;
  });
  box.append(el('span', 'sode__field-label', 'Link'), el('div', 'sode-linkrow__row', input, copy), note, ...(host ? [host] : []));
  return box;
}
