// Settings role tags (docs/agents/settings-saves.md). Keys are the data-section slugs in index.html. Pure (Node-tested).

const tag = (edit, lower) => ({ edit, lower });
const PREF = tag('anyone', 'shown');
const TRUSTED = tag('trusted', 'shown');
const ADMIN_SHOWN = tag('administrator', 'shown');
const ADMIN_HIDDEN = tag('administrator', 'hidden');

export const SECTION_ROLES = Object.freeze({
  'this-browser': PREF,
  'my-user': PREF,
  'everyone-defaults': TRUSTED,
  branding: TRUSTED,
  history: TRUSTED,
  'level-colors': ADMIN_SHOWN,
  confirm: ADMIN_SHOWN,
  routers: ADMIN_SHOWN,
  'router-matrix': ADMIN_SHOWN,
  'router-names': ADMIN_SHOWN,
  takes: ADMIN_SHOWN,
  protects: ADMIN_HIDDEN,
  locks: ADMIN_HIDDEN,
  users: ADMIN_HIDDEN,
  api: ADMIN_HIDDEN,
  panels: ADMIN_HIDDEN,
  proxy: ADMIN_HIDDEN,
  'export-import': ADMIN_HIDDEN,
  'run-setup-again': ADMIN_HIDDEN,
});

// lockState's role values; '' (a held Lock) and anything unknown rank as a Standard user.
const RANK = { anyone: 0, standard: 1, trusted: 2, administrator: 3 };
const rank = (role) => RANK[role] || RANK.standard;

// 'edit', 'read' (a Shown section, changed through an Approval) or 'hidden'. The phone keeps only what Anyone edits,
// the Preferences topic.
export function sectionView(slug, role, phone = false) {
  const t = SECTION_ROLES[slug];
  if (!t) return 'hidden';
  if (t.edit === 'anyone') return 'edit';
  if (phone) return 'hidden';
  if (rank(role) >= RANK[t.edit]) return 'edit';
  return t.lower === 'shown' ? 'read' : 'hidden';
}

export const ROLE_WHO = { trusted: 'a Trusted user or an Administrator', administrator: 'an Administrator' };
export const ROLE_BADGE = { trusted: 'Trusted', administrator: 'Administrator' };

// The highest Role these sections need, in words.
export const approvalWho = (slugs) => ROLE_WHO[slugs.some((s) => SECTION_ROLES[s]?.edit === 'administrator') ? 'administrator' : 'trusted'];
