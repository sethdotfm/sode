// Protects and Router locks on a destination (GLOSSARY.md). Pure, no imports, so Node tests them. prot is S.prot:
// dst → Map(level → {dev, kind}); by is S.protBy: dst → {by, device, levels}, Sode's own record.

// The router's kinds; keep in step with swp08.KindProtect and swp08.KindRouterLock.
export const KIND_PROTECT = 1;
export const KIND_ROUTER_LOCK = 2;

// One hold of a destination: Sode's own (with who set it), another device's Protect, or a Router lock.
const holdKey = (h) => `${h.sode ? 's' : h.kind === KIND_ROUTER_LOCK ? 'l' : 'p'}${h.device}`;

// device: Sode's device number for the router (0 is off). The record's device counts too, for a Protect set before
// the number changed.
export function holdsOn(prot, by, dst, levels, device) {
  const t = prot.get(dst);
  const rec = by.get(dst) || null;
  const hit = t ? levels.filter((u) => t.has(u)) : [];
  const groups = new Map();
  for (const u of hit) {
    const { dev, kind } = t.get(u);
    const sode = kind === KIND_PROTECT && ((device > 0 && dev === device) || (rec && dev === rec.device));
    const h = { kind, device: dev, sode, by: sode ? rec?.by || '' : '', levels: [] };
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

// "Protected by Sode (Ana)", "Protected by 112", "Router lock by 9".
export function holdLabel(h) {
  if (h.sode) return h.by ? `Protected by Sode (${h.by})` : 'Protected by Sode';
  return `${h.kind === KIND_ROUTER_LOCK ? 'Router lock' : 'Protected'} by ${h.device}`;
}

export const protectedText = (p) => p.holds.map(holdLabel).join('; ');

// What the route view offers for p (holdsOn of every level): 'set', 'clear' or ''. A hold Sode did not set gets no
// button. right: the signed-in Rights allow a Protect; device: Sode's device number, 0 when off.
export function protectOffer(p, { right, device, linked }) {
  if (!linked || !right || p.others.length) return '';
  if (p.sode) return 'clear';
  return device > 0 && !p.levels.length ? 'set' : '';
}

// The Protect right of the browser's sign-in. rights: myRights, or null with nobody signed in. Then only a Sode with
// no users (every browser an Administrator) has it. Keep in step with AllowsProtect on the server.
export const protectRight = (rights, role, users) => (rights ? rights.protect === true : role === 'administrator' && !users?.length);

// The Protect picker's rows (protectpick.js): the destinations in dsts where offerOf(n) is want ('set' or 'clear') and
// inRights(n) is true, in the list's order.
export const pickRows = (dsts, want, offerOf, inRights) => dsts.filter((d) => inRights(d.n) && offerOf(d.n) === want);

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
