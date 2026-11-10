// The current Page per panel in this browser (localStorage sode-panel-page-<panel id>, docs/persisted-names.md), so a
// reload or a rebooted tablet reopens it. Storage that throws or is empty reads as no Page.
const keyOf = (id) => `sode-panel-page-${id}`;

export function savedPage(id) {
  try {
    return localStorage.getItem(keyOf(id)) || '';
  } catch {
    return '';
  }
}

export function keepPage(id, page) {
  try {
    localStorage.setItem(keyOf(id), page);
  } catch {
    // Private mode or full storage: the panel still works and reopens on the first enabled Page.
  }
}
