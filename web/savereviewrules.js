// The one Save review (docs/agents/settings-saves.md): the was/now diff, its grouping, the Save button and the marks
// on refused fields. Pure, no imports (Node-tested).

// Each field the one Save sends, and its section slug. Keep in step with saveFields in internal/app/onesave.go
// (TestSaveFieldsMatchPage).
export const SAVE_FIELDS = Object.freeze({
  browserDefaults: 'everyone-defaults',
  brandText: 'branding',
  brandBorder: 'branding',
  hiddenDsts: 'history',
  hiddenDstsRouter: 'history',
  levelColors: 'level-colors',
  confirmLockoutMs: 'confirm',
  resultDismissSeconds: 'confirm',
  confirmEnter: 'confirm',
  router: 'router-matrix',
  routerName: 'router-matrix',
  routerShow: 'router-matrix',
  matrix: 'router-matrix',
  levels: 'router-matrix',
  levelWireOffset: 'router-matrix',
  levelNumbering: 'router-matrix',
  numberOffset: 'router-matrix',
  nameRereadSeconds: 'router-names',
  nameLength: 'router-names',
  nameRequests: 'router-names',
  defaultLevels: 'takes',
  tallyResyncSeconds: 'takes',
  fireMode: 'takes',
  salvoGroup: 'takes',
  pinWaits: 'locks',
  locks: 'locks',
  autoLockMinutes: 'locks',
  trustedProxies: 'proxy',
  requireHttps: 'proxy',
});

// The router connection fields (guard:"router" in internal/config/router.go). A change reconnects the router.
const RECONNECT = new Set(['router', 'matrix', 'levels', 'levelWireOffset', 'nameLength', 'nameRequests']);
export const reconnects = (key) => RECONNECT.has(key);

// Critical fields of the form (criticalEdits in internal/app/recheck.go). The page asks the PIN recheck before it
// sends them; a server refusal still asks if this list falls behind.
const CRITICAL = new Set([...RECONNECT, 'locks', 'autoLockMinutes', 'pinWaits']);
export const critical = (key) => CRITICAL.has(key);

// Lists that read as one row per item.
const INDEXED = new Set(['levelColors']);

const plain = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const same = (a, b) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null);

// One row per change between the form as opened (base) and as it is now. A key now leaves out is not a change.
export function diffRows(base, now) {
  const rows = [];
  for (const key of Object.keys(now)) {
    const was = base[key];
    const is = now[key];
    if (same(was, is)) continue;
    if (plain(was) && plain(is)) {
      for (const sub of new Set([...Object.keys(was), ...Object.keys(is)])) {
        if (!same(was[sub], is[sub])) rows.push({ key, sub, was: was[sub], now: is[sub] });
      }
    } else if (INDEXED.has(key) && Array.isArray(was) && Array.isArray(is)) {
      for (let i = 0; i < Math.max(was.length, is.length); i++) {
        if ((was[i] || '') !== (is[i] || '')) rows.push({ key, sub: i, was: was[i] || '', now: is[i] || '' });
      }
    } else rows.push({ key, sub: null, was, now: is });
  }
  return rows;
}

// placeOf(row) -> {topic, section, order}; order is the section's place on the page.
export function groupRows(rows, placeOf) {
  const topics = [];
  const placed = rows.map((r) => ({ r, p: placeOf(r) })).sort((a, b) => a.p.order - b.p.order);
  for (const { r, p } of placed) {
    let t = topics.at(-1);
    if (t?.topic !== p.topic) topics.push((t = { topic: p.topic, sections: [] }));
    let s = t.sections.at(-1);
    if (s?.section !== p.section) t.sections.push((s = { section: p.section, rows: [] }));
    s.rows.push(r);
  }
  return topics;
}

export function saveLabel(save, keep) {
  if (!save) return 'Nothing can save';
  if (keep) return `Save ${save}, keep ${keep} unsaved`;
  return `Save ${save} ${save === 1 ? 'change' : 'changes'}`;
}

export function showValue(v) {
  if (v === true) return 'On';
  if (v === false) return 'Off';
  if (v === '' || v == null || (Array.isArray(v) && !v.length)) return 'Blank';
  if (Array.isArray(v)) return v.join(', ');
  return String(v);
}

const upper = (s) => s.charAt(0).toUpperCase() + s.slice(1);

// r: a refused field's result {reason, err, value}; show formats the other person's value.
export function refusalNote(r, show) {
  switch (r.reason) {
    case 'conflict':
      return `Not saved. Someone else saved "${show(r.value)}" after you opened Settings.`;
    case 'Role':
      return 'Not saved. Your Role cannot change this.';
    case 'needs Approval':
      return 'Not saved. This needs Approval.';
    case 'needs PIN recheck':
      return 'Not saved. This needs a PIN recheck.';
  }
  const why = String(r.err || r.reason || '').replace(/\.$/, '');
  return why ? `Not saved. ${upper(why)}.` : 'Not saved.';
}
