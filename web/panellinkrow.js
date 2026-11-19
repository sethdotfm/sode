import { el, icon, once, S } from './core.js';
import { linkFor } from './panellink.js';
import { panelLink } from './itemlink.js';
import { qrNode } from './keyview.js';
import { isPublic } from './panelsrules.js';
import { linkBase, shareHostField } from './sharehost.js';

const COPIED_MS = 2000;

// False when the browser refused (no secure context, no permission).
export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}

export const itemLink = (panel) => {
  const link = linkFor(panel, { base: linkBase() });
  return S.installUid ? panelLink(link, S.installUid) : link;
};

// withHost false leaves out the address field and the QR code; the Share modal still offers them.
export function linkRow(panel, { withHost = true } = {}) {
  const box = el('div', 'sode__field sode-linkrow');
  if (panel.acts === 'api') {
    box.append(el('p', 'sode__hint', 'API panel: the link needs a key (Share > Link with key).'));
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
  if (panel.acts === 'both') note.textContent = 'Asks for a sign-in. Keyed links: Share.';
  if (isPublic(panel)) note.textContent = 'Opens for anyone. No sign-in.';
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
    if (withHost) qrBox.replaceChildren(qrNode(link, 'QR code of the Item link') || el('p', 'sode__hint', 'Too long for a QR code.'));
  };
  redraw();
  if (!S.installUid) once('helloData', redraw);
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
