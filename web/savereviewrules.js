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
  routerShow: 'routers',
  pinWaits: 'locks',
  locks: 'locks',
  autoLockMinutes: 'locks',
  guestRole: 'locks',
  newUserDefaults: 'users',
  publicPanels: 'panels',
  newPanelActs: 'panels',
  tieHopWeight: 'tie-lines',
  trustedProxies: 'proxy',
  requireHttps: 'proxy',
  maxPinLen: 'users',
  guestRights: 'users',
  api: 'api',
  guestApiScopes: 'api',
});

// Item keys, `user/<id>/<attribute>`, `client/<name>/<attribute>` and `router/<uid>/<attribute>`, by their first part.
// Keep in step with internal/app/onesavefields.go.
const ITEM_SECTIONS = Object.freeze({ user: 'users', client: 'users', swap: 'routers', router: 'routers' });
// The section of each router key router/<uid>/<attribute>, every router the same: each is in the detail view of its
// router. Keep in step with routerAttrs in internal/app/onesavefields.go (TestSaveFieldsMatchPage).
export const ROUTER_ATTRS = Object.freeze({
  prefix: 'routers',
  title: 'routers',
  address: 'routers',
  matrix: 'routers',
  levels: 'routers',
  levelWireOffset: 'routers',
  levelNumbering: 'routers',
  defaultLevels: 'routers',
  numberOffset: 'routers',
  nameRereadSeconds: 'routers',
  nameLength: 'routers',
  nameRequests: 'routers',
  tallyResyncSeconds: 'routers',
  fireMode: 'routers',
  salvoGroup: 'routers',
  protectDevice: 'routers',
});
const routerAttrOf = (key) => (key.startsWith('router/') ? key.slice(key.lastIndexOf('/') + 1) : '');
export const sectionOfKey = (key) => SAVE_FIELDS[key] || ROUTER_ATTRS[routerAttrOf(key)] || ITEM_SECTIONS[key.split('/')[0]];

// The router connection fields of a router (reconnect:"true" in internal/config/router.go). A change reconnects that
// router.
const ROUTER_RECONNECT = /^router\/[^/]+\/(levels|levelWireOffset|address|matrix|nameLength|nameRequests)$/;
// Add router and Remove router change the router connections too.
const ROUTER_COUNT = /^router\/[^/]+\/(add|remove)$/;
export const reconnects = (key) => ROUTER_RECONNECT.test(key);

// reconnectTargets: the routers that the keys reconnect, in list order, each {uid, name, address}. routers is the
// hello's list, the first router first. nameOf(router) gives its shown name and addressOf(uid) the address it will
// have after the save.
export function reconnectTargets(keys, routers, nameOf, addressOf) {
  const uids = new Set(keys.filter(reconnects).map((k) => k.split('/')[1]));
  const list = routers.length ? routers : [{ uid: '' }];
  return list.filter((r) => uids.has(r.uid || '')).map((r) => ({ uid: r.uid || '', name: nameOf(r), address: addressOf(r.uid || '') }));
}

// The Save and reconnect question. targets: reconnectTargets; many: there are two or more routers, so each is named.
export function reconnectQuestion(targets, many) {
  const at = (t) => t.address || 'no address';
  if (!many) return { title: `Save and reconnect${targets[0]?.address ? ` to ${targets[0].address}` : ''}?`, lines: [] };
  if (targets.length === 1) return { title: `Save and reconnect ${targets[0].name} to ${at(targets[0])}?`, lines: [] };
  return { title: `Save and reconnect ${targets.length} routers?`, lines: targets.map((t) => `${t.name} to ${at(t)}`) };
}

// Critical fields of the form (criticalEdits in internal/app/recheck.go). The page asks the PIN recheck before it
// sends them; a server refusal still asks if this list falls behind.
const CRITICAL = new Set(['locks', 'autoLockMinutes', 'guestRole', 'pinWaits', 'publicPanels', 'newPanelActs', 'newUserDefaults', 'guestApiScopes']);
// Rights are Critical only when wider than the saver's own, which the server decides.
const CRITICAL_ATTRS = /^user\/.+\/(role|pin|persistent|noAutoLock|remove|add)$/;
export const critical = (key) => CRITICAL.has(key) || CRITICAL_ATTRS.test(key) || ROUTER_RECONNECT.test(key) || ROUTER_COUNT.test(key);

// Lists that read as one row per item.
const INDEXED = new Set(['levelColors']);
// Objects that read as one row: Rights.
const whole = (key) => key === 'guestRights' || key.includes('/');

const plain = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const same = (a, b) => JSON.stringify(a ?? null) === JSON.stringify(b ?? null);

// One row per change between the form as opened (base) and as it is now. A key now leaves out is not a change.
export function diffRows(base, now) {
  const rows = [];
  for (const key of Object.keys(now)) {
    const was = base[key];
    const is = now[key];
    if (same(was, is)) continue;
    if (plain(was) && plain(is) && !whole(key)) {
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
  if (!save) return 'Nothing to save';
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
      return 'Not saved. Your Role is not allowed to change this.';
    case 'needs Approval':
      return 'Not saved. This needs Approval.';
    case 'needs PIN recheck':
      return 'Not saved. This needs a PIN recheck.';
  }
  const why = String(r.err || r.reason || '').replace(/\.$/, '');
  return why ? `Not saved. ${upper(why)}.` : 'Not saved.';
}
