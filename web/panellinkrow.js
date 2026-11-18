// A panel's Item link as a read-only field with a Copy link button, for the editor's Identity section and the Share
// modal. The Share modal also shows its QR code. The text comes from linkFor and itemlink.js only. A panel that only acts as its API client has no link without its key. A
// Both panel's plain link opens in Session mode, with the keyed links under Share. A Public panel's opens for anyone.
import { el, icon, on, S } from './core.js';
import { linkFor } from './panellink.js';
import { panelLink } from './itemlink.js';
import { qrNode } from './keyview.js';
import { isPublic } from './panelsrules.js';
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

// Before the hello the install is not known, so the link has no install until it comes.
export const itemLink = (panel) => {
  const link = linkFor(panel, { base: linkBase() });
  return S.installUid ? panelLink(link, S.installUid) : link;
};

// withHost false leaves out the address-for-other-devices field and the QR code; the Share modal still offers them.
export function linkRow(panel, { withHost = true } = {}) {
  const box = el('div', 'sode__field sode-linkrow');
  if (panel.acts === 'api') {
    box.append(el('p', 'sode__hint', 'This panel acts as its own API client, so its link needs the key. Use Link with key in Share.'));
    return box;
  }
  let link = itemLink(panel);
  const input = el('input', 'pb-input sode-linkrow__text');
  input.readOnly = true;
  input.spellcheck = false;
  input.value = link;
  input.setAttribute('aria-label', 'Item link');
  input.addEventListener('focus', () => input.select());
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  if (panel.acts === 'both') note.textContent = 'Asks for a sign-in. A link with a key is under Share.';
  if (isPublic(panel)) note.textContent = 'Opens for anyone with the link, with no sign-in.';
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
  const qrBox = el('div', 'sode-linkrow__qr');
  const redraw = () => {
    link = itemLink(panel);
    input.value = link;
    if (withHost)
      qrBox.replaceChildren(qrNode(link, 'QR code of the Item link') || el('p', 'sode__hint', 'This link is too long for a QR code. Copy it instead.'));
  };
  redraw();
  if (!S.installUid) {
    const off = on('helloData', () => {
      queueMicrotask(off); // a removal inside emit would skip the next listener
      redraw();
    });
  }
  const host = withHost ? shareHostField(redraw) : null;
  box.append(
    el('span', 'sode__field-label', 'Item link'),
    el('div', 'sode-linkrow__row', input, copy),
    note,
    ...(host ? [host] : []),
    ...(withHost ? [qrBox] : []),
  );
  return box;
}
