// The Palette's setting rows. Home reads the Settings form on the page. Other pages read a copy of index.html one time,
// when the Palette first opens there, so each page lists the same settings.
import { sectionSettings } from './settingsfind.js';

// [{ section, sectionTitle, topic, id, label, button }] for the catalogue (palettecatalogue.js). live: doc is this page,
// so a [data-find] control that is hidden now (Sign in while signed in) is left out, as Settings search does. button is
// the id of the button that the setting is, or ''. A field that holds one button with its own id (Clear History) is
// that button.
export function settingsEntries(doc, live) {
  const out = [];
  for (const sec of doc.querySelectorAll('fieldset[data-section]')) {
    const title = sec.querySelector('legend')?.firstChild?.textContent.trim() || '';
    const topic = doc.querySelector(`.sode__topic[data-topic="${sec.dataset.topic}"]`)?.textContent.trim() || '';
    for (const s of sectionSettings(sec)) {
      if (live && s.node.matches('[data-find]') && s.node.closest('[hidden]')) continue;
      const b = s.node.tagName === 'BUTTON' ? s.node : s.node.querySelector(`button[id="${s.id}"]`);
      out.push({ section: sec.dataset.section, sectionTitle: title, topic, id: s.id, label: s.label, button: b?.id || '' });
    }
  }
  return out;
}

let fetched = null;

// Resolves to the entries; [] when the copy cannot be read.
export function settingsRows() {
  const form = document.getElementById('settings-view');
  if (form) return Promise.resolve(settingsEntries(document, true));
  fetched ||= fetch(new URL('./', document.baseURI))
    .then((r) => (r.ok ? r.text() : ''))
    .then((html) => (html ? settingsEntries(new DOMParser().parseFromString(html, 'text/html'), false) : []))
    .catch(() => []);
  return fetched;
}
