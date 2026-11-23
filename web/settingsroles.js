// The server still decides every save.
import { $, el, on } from './core.js';
import { layout, onLayout } from './layout.js';
import { sectionView, SECTION_ROLES, ROLE_BADGE, ROLE_WHO } from './settingsrolerules.js';
import { desktopOnly } from './desktoponly.js';

// Each legend shows the Role that edits it; a lower Role's section stays editable and Save asks for an Approval.
function badges(s) {
  const edit = SECTION_ROLES[s.dataset.section]?.edit;
  if (!ROLE_BADGE[edit]) return null;
  const approval = el('span', 'pb-badge pb-badge--warn', 'Approval');
  approval.dataset.pbTip = `Save needs Approval from ${ROLE_WHO[edit]}`;
  s.querySelector(':scope > legend').append(' ', el('span', `pb-badge sode__role-badge sode__role-badge--${edit}`, ROLE_BADGE[edit]), ' ', approval);
  return approval;
}

export function initRoles({ form, sections, toc, isOpen }) {
  let role = '';
  const phoneNote = desktopOnly('change them', 'Other settings need a computer.');
  phoneNote.id = 'settings-desktop-only';
  $('settings-toc').after(phoneNote);

  const approvals = new Map(sections.map((s) => [s, badges(s)]));

  function apply() {
    const phone = layout() === 'phone';
    const off = [];
    sections.forEach((s, i) => {
      const view = sectionView(s.dataset.section, role, phone);
      s.hidden = view === 'hidden';
      if (s.hidden) off.push(i);
      const approval = approvals.get(s);
      if (approval) approval.hidden = view !== 'read';
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
