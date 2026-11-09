// Icons draw from symbols copied into the page, never from a <use> of the external sprite: Chrome loads and lays out
// its own copy of lucide.svg (370 KB, about 16,700 nodes) for every <use> made against it, freed only at a major GC,
// so a panel that redrew a tick each second climbed past 4 GB (TestChromeRunPageHoldsNoMemory). Imports nothing.

export const SPRITE = 'vendor/patchbay/icons/lucide.svg';
const NS = 'http://www.w3.org/2000/svg';
const PREFIX = 'pb-i-'; // the page's own ids never collide with an icon name

export const localHref = (name) => `#${PREFIX}${name}`;
export const nameOfHref = (href) => (String(href).startsWith(`#${PREFIX}`) ? String(href).slice(PREFIX.length + 1) : '');

// The one <symbol> for name, renamed to its local id; '' when the sprite has none.
export function symbolMarkup(sprite, name) {
  if (!/^[a-z0-9-]+$/.test(name)) return '';
  const m = new RegExp(`<symbol id="${name}"([\\s>][\\s\\S]*?</symbol>)`).exec(sprite);
  return m ? `<symbol id="${PREFIX}${name}"${m[1]}` : '';
}

let text = null; // Promise<string> of the sprite, read once
let defs = null;
const asked = new Set();

// The href for a <use>; the symbol joins the page once the sprite is read, and the <use> draws it then.
export function iconHref(name, doc = document) {
  if (!asked.has(name)) {
    asked.add(name);
    text ??= fetch(SPRITE).then((r) => (r.ok ? r.text() : Promise.reject(new Error(String(r.status)))));
    text.then(
      (t) => {
        const m = symbolMarkup(t, name);
        if (!m) return;
        if (!defs?.isConnected) {
          defs = doc.createElementNS(NS, 'svg');
          defs.setAttribute('aria-hidden', 'true');
          defs.setAttribute('width', '0');
          defs.setAttribute('height', '0');
          defs.style.position = 'absolute';
          (doc.body || doc.documentElement).append(defs);
        }
        // Parsed in the <svg>'s context, so no extra document is made for it.
        defs.insertAdjacentHTML('beforeend', m);
      },
      () => {
        // Not read: the next icon asks again.
        text = null;
        asked.delete(name);
      },
    );
  }
  return localHref(name);
}

// Static markup (index.html) writes <use href="#pb-i-name">; this brings in its symbols.
export function adoptIcons(root = document) {
  for (const u of root.querySelectorAll(`use[href^="#${PREFIX}"]`)) iconHref(nameOfHref(u.getAttribute('href')), root.ownerDocument || root);
}
