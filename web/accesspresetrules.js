// Pure, no imports (Node-tested). A preset is a usersList accessPresets entry (internal/app/accesspreset.go).

const plain = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
// Scopes come back in the page's order, so a list compares as a set.
const norm = (v) => (Array.isArray(v) ? [...v].sort() : (v ?? null));
const same = (a, b) => JSON.stringify(norm(a)) === JSON.stringify(norm(b));

// ticked: only those user rows change.
export function presetTarget(preset, ticked) {
  const out = structuredClone(preset.fields);
  for (const id of ticked) {
    out[`user/${id}/persistent`] = !!preset.users.persistent;
    out[`user/${id}/noAutoLock`] = !!preset.users.noAutoLock;
  }
  return out;
}

// One leaf per diffRows row.
function leaves(target) {
  const out = [];
  for (const [key, v] of Object.entries(target)) {
    if (plain(v)) for (const [sub, x] of Object.entries(v)) out.push({ key, sub, want: x });
    else out.push({ key, sub: null, want: v });
  }
  return out;
}
const leafId = (l) => `${l.key}/${l.sub}`;

// A mark stays only while its row is unchanged, so a hand edit after Apply drops it.
export function presetMarks(marks, rows, name, target) {
  const want = new Map(target ? leaves(target).map((l) => [leafId(l), l.want]) : []);
  const out = new Map();
  for (const r of rows) {
    const id = `${r.key}/${r.sub}`;
    const m = marks.get(id);
    if (name && want.has(id) && same(want.get(id), r.now)) out.set(id, { preset: name, now: r.now });
    else if (m && same(m.now, r.now)) out.set(id, m);
  }
  return out;
}

export function presetGaps(target, live) {
  const out = [];
  for (const l of leaves(target)) {
    const v = l.sub === null ? live[l.key] : live[l.key]?.[l.sub];
    if (v !== undefined && !same(v, l.want)) out.push({ ...l, live: v });
  }
  return out;
}

const PAGES = { main: 'The Main page', log: 'The Communication Log', panel: 'The panel pages' };
const ACTS = { public: 'Public', session: 'User', both: 'Both', api: 'API' };

export function gapText(g) {
  const v = g.live;
  switch (g.key) {
    case 'locks':
      return `${PAGES[g.sub] || g.sub} ${v ? 'asks for a PIN' : 'opens with no PIN'}`;
    case 'autoLockMinutes':
      return v ? `Auto-lock after ${v} min` : 'Auto-lock: off';
    case 'guestRole':
      return `Guest role: ${v === 'trusted' ? 'Trusted' : 'Standard'}`;
    case 'pinWaits':
      return `Wait after wrong PINs: ${v ? 'on' : 'off'}`;
    case 'publicPanels':
      return v ? 'Public panels work' : 'No panel can be Public';
    case 'newPanelActs':
      return `A new panel starts in ${ACTS[v] || v} mode`;
    case 'api':
      return `Keyless API: ${v ? 'on' : 'off'}`;
    case 'guestApiScopes':
      return `Keyless takes: ${v.includes('take') ? 'on' : 'off'}`;
    case 'newUserDefaults':
      return `New user defaults: ${g.sub === 'persistent' ? 'Never sign out' : 'No auto-lock'} ${v ? 'on' : 'off'}`;
  }
  const attr = g.key.split('/')[2];
  if (attr === 'persistent') return `Never sign out: ${v ? 'on' : 'off'}`;
  if (attr === 'noAutoLock') return `No auto-lock: ${v ? 'on' : 'off'}`;
  return '';
}
