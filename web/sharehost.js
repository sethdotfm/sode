// Links and QR codes made on a page opened as localhost are useless on a phone. This swaps in the address other
// devices use, typed once and kept in this browser (`sode-share-host`). The rules are in sharehostrules.js.
import { el } from './core.js';
import { isLoopback, parseShareHost, shareBase } from './sharehostrules.js';

const KEY = 'sode-share-host';
export function storedShareHost() {
  try {
    return localStorage.getItem(KEY) || '';
  } catch {
    return ''; // blocked storage only costs the memory of the address
  }
}

function keepShareHost(text) {
  try {
    if (text) localStorage.setItem(KEY, text);
    else localStorage.removeItem(KEY);
  } catch {
    // see storedShareHost
  }
}

export const linkBase = () => shareBase(document.baseURI, storedShareHost());

// A field above a link for a page opened as localhost, or null elsewhere. onChange runs after a valid address or a clear.
export function shareHostField(onChange) {
  if (!isLoopback(location.hostname)) return null;
  const input = el('input', 'pb-input');
  input.id = 'share-host';
  input.autocomplete = 'off';
  input.spellcheck = false;
  input.placeholder = '192.168.1.20:8080';
  input.value = storedShareHost();
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const show = () => {
    note.textContent = input.value && !parseShareHost(input.value, location.protocol) ? 'That is not an address another device can reach.' : '';
  };
  show();
  input.addEventListener('input', () => {
    show();
    if (input.value.trim() === '' || parseShareHost(input.value, location.protocol)) {
      keepShareHost(input.value.trim());
      onChange();
    }
  });
  return el(
    'div',
    'sode__field',
    Object.assign(el('label', null, 'Address for other devices'), { htmlFor: 'share-host' }),
    el(
      'span',
      'sode__hint',
      'This page is open on localhost, which a phone cannot reach. Type the address it uses, with the port. It is kept in this browser.',
    ),
    input,
    note,
  );
}
