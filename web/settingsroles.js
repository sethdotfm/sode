// The Settings role tags (settingsrolerules.js) on the page. A section is editable, read-only or hidden for this
// connection's Role. The phone keeps only Preferences. The server still decides every save.
import { $, el, on } from './core.js';
import { layout, onLayout } from './layout.js';
import { sectionView, SECTION_ROLES } from './settingsrolerules.js';
import { desktopOnly } from './desktoponly.js';

const WHO = { trusted: 'a Trusted user or an Administrator', administrator: 'an Administrator' };

export function initRoles({ form, sections, toc, isOpen }) {
  let role = '';
  const phoneNote = desktopOnly('change them', 'Other settings need a computer.');
  phoneNote.id = 'settings-desktop-only';
  $('settings-toc').after(phoneNote);

  function apply() {
    const phone = layout() === 'phone';
    const off = [];
    sections.forEach((s, i) => {
      const view = sectionView(s.dataset.section, role, phone);
      s.hidden = view === 'hidden';
      if (s.hidden) off.push(i);
      // A read section stays editable: Save asks for an Approval for its changes.
      let note = s.querySelector(':scope > .sode__role-note');
      if (view === 'read' && !note) {
        note = el('p', 'sode__hint sode__role-note', `Changes here need Approval from ${WHO[SECTION_ROLES[s.dataset.section].edit]}.`);
        s.querySelector(':scope > legend').after(note);
      }
      if (note) note.hidden = view !== 'read';
    });
    // A band or router sub-heading goes with its last shown section.
    for (const h of form.querySelectorAll(':scope > .sode__topic, :scope > .sode__router')) {
      const mine = h.matches('.sode__topic') ? (s) => s.dataset.topic === h.dataset.topic : (s) => s.dataset.router === h.dataset.router;
      h.hidden = !sections.some((s) => mine(s) && !s.hidden);
    }
    phoneNote.hidden = !phone;
    toc.setOff(off);
  }

  on('access', (a) => {
    role = a.role || '';
    if (isOpen()) apply();
  });
  onLayout(() => apply());
  apply();
  return { apply };
}
