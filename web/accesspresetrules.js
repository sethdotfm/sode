// Apply an access preset (docs/agents/settings-saves.md): what it stages, the marks with its name, and what still
// differs after a partly refused save. Pure, no imports (Node-tested). A preset is the usersList answer's
// accessPresets entry {key, name, fields, users} (internal/app/accesspreset.go).

const plain = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
// Scopes come back in the page's order, so a list compares as a set.
const norm = (v) => (Array.isArray(v) ? [...v].sort() : (v ?? null));
const same = (a, b) => JSON.stringify(norm(a)) === JSON.stringify(norm(b));

// The one Save keys and values the preset writes. ticked: the ids of the user rows ticked; only they change.
export function presetTarget(preset, ticked) {
  const out = structuredClone(preset.fields);
  for (const id of ticked) {
    out[`user/${id}/persistent`] = !!preset.users.persistent;
    out[`user/${id}/noAutoLock`] = !!preset.users.noAutoLock;
  }
  return out;
}

// One leaf per diffRows row: an object field gives one per key the preset sets, any other field one with sub null.
function leaves(target) {
  const out = [];
  for (const [key, v] of Object.entries(target)) {
    if (plain(v)) for (const [sub, x] of Object.entries(v)) out.push({ key, sub, want: x });
    else out.push({ key, sub: null, want: v });
  }
  return out;
}
const leafId = (l) => `${l.key}/${l.sub}`;

// marks: rowId -> {preset, now}. With a preset, rows that equal its value take its name. A mark stays only while its
// row is unchanged, so a hand edit after Apply drops it.
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

// The leaves of target that live (the values as saved now) does not match: {key, sub, live, want}.
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

// What a gap's live value does now.
export function gapText(g) {
  const v = g.live;
  switch (g.key) {
    case 'locks':
      return `${PAGES[g.sub] || g.sub} ${v ? 'asks for a PIN' : 'opens with no PIN'}`;
    case 'autoLockMinutes':
      return v ? `Pages lock after ${v} minutes with no use` : 'Auto-lock is off';
    case 'guestRole':
      return `A browser with nobody signed in is a ${v === 'trusted' ? 'Trusted' : 'Standard'} user`;
    case 'pinWaits':
      return v ? 'Wrong PINs make a name wait' : 'Wrong PINs do not make a name wait';
    case 'publicPanels':
      return v ? 'Public panels work' : 'No panel can be Public';
    case 'newPanelActs':
      return `A new panel starts in ${ACTS[v] || v} mode`;
    case 'api':
      return v ? 'Apps with no key can use the API' : 'Apps with no key cannot use the API';
    case 'guestApiScopes':
      return v.includes('take') ? 'Apps with no key can make takes' : 'Apps with no key cannot make takes';
    case 'newUserDefaults':
      return `A new user starts with ${g.sub === 'persistent' ? 'Never sign out' : 'No auto-lock'} ${v ? 'on' : 'off'}`;
  }
  const attr = g.key.split('/')[2];
  if (attr === 'persistent') return v ? 'This user never signs out' : 'This user signs out';
  if (attr === 'noAutoLock') return v ? 'Auto-lock skips this user' : 'Auto-lock locks this user';
  return '';
}
