// Protects and Router locks on a destination (GLOSSARY.md). Pure, no imports, so Node tests them. prot is S.prot:
// dst → Map(level → {dev, kind}); by is S.protBy: dst → {by, kind, device, levels}, Sode's own record.

// The router's kinds; keep in step with swp08.KindProtect and swp08.KindRouterLock.
export const KIND_PROTECT = 1;
export const KIND_ROUTER_LOCK = 2;

// One hold of a destination: Sode's own (with who set it), another device's Protect, or a Router lock.
const holdKey = (h) => `${h.sode ? 's' : h.kind === KIND_ROUTER_LOCK ? 'l' : 'p'}${h.device}`;

// The kinds of who set one of Sode's own Protects (protectBy.kind); keep in step with byUser, byGone, byClient and
// byGuest in internal/app/protect.go.
export const BY_USER = 'user';
export const BY_CLIENT = 'client';
export const BY_GUEST = 'guest';

// device: Sode's device number for the router (0 is off). The record's device counts too, for a Protect set before
// the number changed. me: the signed-in user's name ('' for none). Each hold gets an owner. 'you' is Sode's, set by me.
// 'sode' is Sode's, set by someone else. 'other' is another system: a panel, DashBoard or another SW-P-08 client.
export function holdsOn(prot, by, dst, levels, device, me = '') {
  const t = prot.get(dst);
  const rec = by.get(dst) || null;
  const hit = t ? levels.filter((u) => t.has(u)) : [];
  const groups = new Map();
  for (const u of hit) {
    const { dev, kind } = t.get(u);
    const sode = kind === KIND_PROTECT && ((device > 0 && dev === device) || (rec && dev === rec.device));
    const mine = sode && !!me && rec?.kind === BY_USER && rec.by === me;
    const h = {
      kind,
      device: dev,
      sode,
      mine,
      owner: mine ? 'you' : sode ? 'sode' : 'other',
      by: sode ? rec?.by || '' : '',
      client: sode && rec?.kind === BY_CLIENT,
      guest: sode && rec?.kind === BY_GUEST,
      levels: [],
    };
    const k = holdKey(h);
    if (!groups.has(k)) groups.set(k, h);
    groups.get(k).levels.push(u);
  }
  const holds = [...groups.values()].sort((a, b) => b.sode - a.sode || a.device - b.device);
  return {
    levels: hit,
    devices: [...new Set(holds.map((h) => h.device))].sort((a, b) => a - b),
    holds,
    sode: holds.find((h) => h.sode) || null,
    others: holds.filter((h) => !h.sode),
  };
}

// "Protected by you", "Protected by Ana in Sode", "Protected by API client Desk in Sode", "Protected by a guest in Sode
// (10.0.0.9)", "Protected by another system
// (device 112)", "Router lock by another system (device 9)". The words name the owner first, so the three owners read
// apart at a glance. The badge's icon and color say the same (holdBadge in protects.js).
export function holdLabel(h) {
  if (h.mine) return 'Protected by you';
  if (h.guest) return h.by ? `Protected by a guest in Sode (${h.by})` : 'Protected by a guest in Sode';
  if (h.sode) return h.by ? `Protected by ${h.client ? 'API client ' : ''}${h.by} in Sode` : 'Protected by Sode';
  return `${h.kind === KIND_ROUTER_LOCK ? 'Router lock' : 'Protected'} by another system (device ${h.device})`;
}

// The short words for a Grid name's mark, where room is small: the owner's icon says Sode or another system, and the
// tip holds holdLabel. "Protected by you", "Protected by Bo", "Protected by device 112", "Router lock by device 9".
export function holdShort(h) {
  if (h.mine) return 'Protected by you';
  if (h.guest) return 'Protected by a guest';
  if (h.sode) return h.by ? `Protected by ${h.by}` : 'Protected by Sode';
  return `${h.kind === KIND_ROUTER_LOCK ? 'Router lock' : 'Protected'} by device ${h.device}`;
}

// What the Grid popup's shield does on name n: the offer ('set' or 'clear') and the destinations. A picked name acts on
// every picked destination with the same offer, so one press protects or clears them all. null when n has no offer.
export function shieldTargets(n, picked, offerOf) {
  const offer = offerOf(n);
  if (!offer) return null;
  if (!picked.includes(n)) return { offer, dsts: [n] };
  return { offer, dsts: picked.filter((d) => d === n || offerOf(d) === offer) };
}

// The badge look of one hold. The icon names the mark: shield for every Protect, shield-ban for a Router lock. The
// `lock` icon is the Lock's alone (GLOSSARY.md). The words name the owner, and the status says if you can clear it:
// yours is info, the others are warn, since they stop you.
export const holdLook = (h) => ({ icon: h.kind === KIND_ROUTER_LOCK ? 'shield-ban' : 'shield', status: h.owner === 'you' ? 'info' : 'warn' });

// The chip of a History Protect entry (e.protect 'set' or 'clear'). It is Sode's own Protect or Free Protect, or a
// Protect or Router lock that another system set or freed (e.ext; holdKind in internal/app/protext.go). Free has its
// own icon. History has no owner to tell apart, so every one is warn.
export function entryChip(e) {
  const lock = e.ext && e.holdKind === 'routerLock';
  const set = e.protect === 'set';
  const what = lock ? 'Router lock' : 'Protect';
  return { icon: set ? (lock ? 'shield-ban' : 'shield') : 'shield-off', status: 'warn', text: set ? what : `Free ${what}` };
}

export const protectedText = (p) => p.holds.map(holdLabel).join('; ');

// What the route view offers for p (holdsOn of every level): 'set', 'clear' or ''. A hold Sode did not set gets no
// button. right: the signed-in Rights allow a Protect. admin: an Administrator, who clears any of Sode's Protects
// without the right (protectClear on the server). device: Sode's device number, 0 when off.
export function protectOffer(p, { right, admin = false, device, linked }) {
  if (!linked || p.others.length) return '';
  if (p.sode) return right || admin ? 'clear' : '';
  return right && device > 0 && !p.levels.length ? 'set' : '';
}

// The Protect right of the browser. rights: myRights, a user's or the guest's (pushRightsMu), or null before it comes.
// Then only a Sode with no users (every browser an Administrator) has it. Keep in step with AllowsProtect on the server.
export const protectRight = (rights, role, users) => (rights ? rights.protect === true : role === 'administrator' && !users?.length);

// A Protect History entry e can be undone when two things are true. It is the newest ok Protect entry of its
// destination in hist. And offer (protectOfferOf now) is the opposite of what e did. Else Undo would undo another thing.
export function protectUndoable(e, hist, offer) {
  // A Protect another system set or freed is not Sode's to undo; it still counts as the newest entry below.
  if (!e.ok || !e.protect || e.ext) return false;
  const newest = hist.reduce((m, x) => (x.protect && x.ok && x.dst === e.dst && (!m || x.id > m.id) ? x : m), null);
  if (newest && newest.id !== e.id) return false;
  return offer === (e.protect === 'set' ? 'clear' : 'set');
}

// The Protect picker's filters (protectpick.js), in the order of its chips.
export const PICK_FILTERS = Object.freeze([
  { id: 'all', label: 'All destinations' },
  { id: 'protected', label: 'Currently protected' },
  { id: 'free', label: 'Currently free' },
]);

// The Protect picker's rows: the destinations in dsts in the Rights (inRights(n)) that pass filter, in the list's order.
// held(n): n has a Protect or a Router lock on a level. offerOf(n): 'set', 'clear' or '' (protectOfferOf); a row with
// no offer shows, but cannot be picked.
export function pickRows(dsts, filter, { inRights, held, offerOf }) {
  const out = [];
  for (const d of dsts) {
    if (!inRights(d.n)) continue;
    const h = held(d.n);
    if ((filter === 'protected' && !h) || (filter === 'free' && h)) continue;
    out.push({ ...d, held: h, offer: offerOf(d.n) });
  }
  return out;
}

// What the picked destinations ask: protect (the free ones) and clear (Sode's own Protects). With both, the picker
// protects first, so the safe action is the default.
export function pickSplit(picked, offerOf) {
  const protect = [];
  const clear = [];
  for (const n of picked) {
    const o = offerOf(n);
    if (o === 'set') protect.push(n);
    else if (o === 'clear') clear.push(n);
  }
  return { protect, clear, first: protect.length ? 'set' : clear.length ? 'clear' : '' };
}

// A Shift+click: the pickable rows from index a to index b, both ends in.
export function pickRange(rows, a, b) {
  const [lo, hi] = a <= b ? [a, b] : [b, a];
  return rows
    .slice(Math.max(0, lo), hi + 1)
    .filter((r) => r.offer)
    .map((r) => r.n);
}

// The rows that match q, best first; all rows in order for an empty q. score is fuzzyScore (fuzzy.js).
export function filterRows(rows, q, score) {
  const t = String(q || '')
    .trim()
    .toLowerCase();
  if (!t) return rows.slice();
  return rows
    .map((r, i) => ({ r, i, s: score(r, t) }))
    .filter((m) => m.s >= 0)
    .sort((a, b) => b.s - a.s || a.i - b.i)
    .map((m) => m.r);
}
