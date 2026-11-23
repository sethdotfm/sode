import { el, S, search, on, onMsg, emit, wsSend, off, srcItem, dstItem, labelNode, tallyText, routerData, routerOfItem, LINK_DOWN } from './core.js';
import { protectedText } from './protectrules.js';
import { protectSummary, sendProtect, holdsAt, holdBadge } from './protects.js';
import { levelText, oneLevel, levelBar, parseLevels, levelLabel } from './levelview.js';
import { allowed, planOutside } from './rightsrules.js';
import { bySource, srcRow, routeRow } from './routes.js';
import { fmtClock, pref } from './device.js';
import { failureGroups } from './history.js';
import { resolvePreset } from './presetstore.js';
import { showResult, setResult } from './banner.js';
import { writeJSON, writeStr } from './store.js';
import { onMyRights } from './editorrights.js';
import { nextAfter } from './boxlist.js';
import { strong, destPart, takeQuestion, planItemHead } from './confirmtext.js';
export { showResult, resultNode } from './banner.js';
export { nameStem } from './boxlist.js';

let takeSeq = 0;
const takes = new Map();

// Plan: { verb, items: [{ dst, groups: [{src, levels}], skipped, undo, retry }] }, one item per
// destination; undo/retry are the entries an item undoes or retries. Protection is not kept
// here: planBlocked returns it. A plan can carry title and warnings (shown as is).

const routeItem = (dst, groups, retry = null) => ({ dst, groups, skipped: [], untouched: [], undo: null, retry });

// Failed-take levels the router answered for were not changed by the take (old route, or
// someone else's): undo leaves them alone. Levels with no answer stay in (it is possible
// that they switched). Late levels did change with the take, whatever `now` says.
const untouchedLevels = (e) => new Set(e.ok ? [] : (e.now || []).map(([u]) => u).filter((u) => !e.late?.includes(u)));

function undoItem(e) {
  const groups = new Map();
  const skipped = [];
  const untouched = untouchedLevels(e);
  for (const [u, s] of e.prev) {
    if (!S.levelBy.has(u) || untouched.has(u)) continue;
    if (s < 0) {
      skipped.push(u);
      continue;
    }
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  const known = new Set(e.prev.map(([u]) => u));
  for (const u of e.levels) if (!known.has(u) && !untouched.has(u)) skipped.push(u);
  return {
    dst: dstItem(e.dst, e.dstName, e.router),
    groups: [...groups].map(([s, levels]) => ({ src: srcItem(s, '', e.router), levels: levels.sort((a, b) => a - b) })),
    skipped: skipped.sort((a, b) => a - b),
    untouched: [...untouched].filter((u) => S.levelBy.has(u)).sort((a, b) => a - b),
    undo: e,
    retry: null,
  };
}

// A failed take is a retry (linked to its entry); any other is a redo, sent as a plain new
// take. Needs a real source and a level that still exists.
export const canRedo = (e) => !!e && e.src >= 0 && e.levels.some((u) => S.levelBy.has(u));
const againItem = (e) =>
  routeItem(
    dstItem(e.dst, e.dstName, e.router),
    [{ src: srcItem(e.src, e.srcName, e.router), levels: e.levels.filter((u) => S.levelBy.has(u)).sort((a, b) => a - b) }],
    e.ok ? null : e,
  );

// The Grid popup's staged routes (routesOf, gridpanelstage.js) as one take: one item for each destination, one group
// for each source on it. Route ends are router numbers. A route with box set is the patch boxes' route (One Take,
// gridpopup.js). Then the plan is mixed and each group says if it is staged, so the Confirm keeps the two apart.
// ends gives the items of the route numbers: the Grid popup's are item numbers (routerids.js).
export function gridPlan(routes, warnings, ends = { src: (n) => srcItem(n), dst: (n) => dstItem(n) }) {
  const mixed = routes.some((r) => r.box);
  const by = new Map();
  for (const r of routes) {
    if (!by.has(r.dst)) by.set(r.dst, []);
    const g = { src: ends.src(r.src), levels: r.levels };
    by.get(r.dst).push(mixed ? { ...g, staged: !r.box } : g);
  }
  const plan = { verb: 'grid', warnings, items: [...by].map(([dst, groups]) => routeItem(ends.dst(dst), groups)) };
  return mixed ? { ...plan, mixed } : plan;
}

export const patchPlan = (src, dsts, levels) => ({
  verb: 'patch',
  items: dsts.map((dst) => routeItem(dst, [{ src, levels }])),
});

// The patch the boxes hold, as preset saving wants it: null until Source and a destination are set.
export const boxPatch = (src, dsts, lvl) => (src && dsts.length ? { src, dsts, levels: lvl.empty() ? [] : lvl.levels() || [] } : null);

// What the router does not have is left out and listed in warnings. Validated at save
// (presetProblem), not here.
export function presetPlan(preset, defaults) {
  // The shown names keep a gone number's old name, so the server's Broken mark decides.
  const marked = (kind, n) => (preset.broken || []).some((m) => m?.kind === kind && m?.ref?.n === n);
  const r = resolvePreset(
    preset,
    {
      src: (n) => S.srcBy.has(n) && !marked('source', n),
      dst: (n) => S.dstBy.has(n) && !marked('destination', n),
      level: (u) => S.levelBy.has(u),
    },
    defaults,
  );
  // The Broken banner (brokenbanner.js) names a marked part.
  const warnings = r.missing
    .filter((m) => !marked(m.kind, m.n))
    .map((m) => `${m.kind === 'level' ? `Level ${m.n}` : `${m.kind[0].toUpperCase()}${m.kind.slice(1)} ${m.n + off()}`}: not on the router`);
  return {
    verb: 'preset',
    preset: preset.name,
    presetId: preset.id,
    title: `Preset: ${preset.name}`,
    warnings,
    items: r.routes.map((t) =>
      routeItem(
        dstItem(t.dst),
        t.groups.map((g) => ({ src: srcItem(g.src), levels: g.levels })),
      ),
    ),
  };
}
// Follows are left out of an undo unless extra.on (shared by copies of the plan). A destination
// followed more than once goes back to before the oldest follow: one undo each, not several
// that race.
function withFollows(plan, follows) {
  const oldest = new Map();
  for (const f of follows) if (!oldest.has(f.dst) || f.id < oldest.get(f.dst).id) oldest.set(f.dst, f);
  return oldest.size ? { ...plan, extra: { items: [...oldest.values()].map(undoItem), on: false } } : plan;
}
export const restorePlan = (then, at) => ({ verb: 'undo', items: [undoItem(then)], at });
export const undoPlan = (e) => withFollows({ verb: 'undo', items: [undoItem(e)] }, e.follows || []);
// A take's plans; a Protect row's Undo is its own (protects.js PROTECT_ENTRY, PROTECT_GROUP).
export const groupUndoPlan = (f) =>
  withFollows(
    { verb: 'undo', items: f.entries.map(undoItem) },
    f.entries.flatMap((e) => e.follows || []),
  );
// A group with failed takes retries only those; otherwise it redoes every redoable take. Empty
// items = nothing to offer. ofUndo marks a redo of an undo row, which leaves Last alone.
export function againPlan(row) {
  const es = row.f ? row.f.entries : [row.e];
  const failed = es.filter((e) => !e.ok);
  const use = (failed.length ? failed : es).filter(canRedo);
  const plan = { verb: failed.length ? 'retry' : 'redo', items: use.map(againItem) };
  return use.some((e) => e.undo) ? { ...plan, ofUndo: true } : plan;
}

// The one table of verb wording. busy: banner while it sends; resets: whether a send clears the
// patch boxes (only a new patch does).
export const VERBS = {
  patch: { word: 'take', ask: 'take', busy: 'Taking…', resets: true },
  undo: { word: 'undo', ask: 'undo', busy: 'Undoing…', resets: false },
  // Put a tie line back: the Undo of a hop entry (historyrows.js putBacks).
  putback: { word: 'put back', ask: 'put back', busy: 'Putting back…', resets: false },
  retry: { word: 'retry', ask: 'retry the take of', busy: 'Retrying…', resets: false },
  redo: { word: 'redo', ask: 'redo the take of', busy: 'Redoing…', resets: false },
  preset: { word: 'take', ask: 'take', busy: 'Taking…', resets: false },
  // The Grid popup's staged routes (gridpopup.js). They are not the patch boxes, so a send leaves the boxes alone.
  grid: { word: 'take', ask: 'take', busy: 'Taking…', resets: false },
  // A Protect is no take: protects.js sends it (plan.protect).
  protect: { word: 'protect', ask: 'protect', busy: 'Protecting…', resets: false },
  unprotect: { word: 'free protect', ask: 'free the Protect on', busy: 'Freeing…', resets: false },
};
export const planVerb = (plan) => VERBS[plan.verb].word;
export const planEmpty = (plan) => !plan.protect && !plan.items.some((i) => i.groups.length);

// "levels 1–4" of a route, or "all levels" on a router with one level.
const routeLevels = (list, uid) => (oneLevel(uid) ? 'all levels' : `levels ${levelText(list, uid)}`);

function routeText(t, already = []) {
  const uid = routerOfItem(t.dst);
  return (
    `${t.g.src.label} → ${t.dst.label} (${routeLevels(t.g.levels, uid)}` +
    (already.length ? `; ${levelText(already, uid)} ${already.length === 1 ? 'was' : 'were'} already on ${t.g.src.label}` : '') +
    ')'
  );
}

function capped(list, n = 3) {
  return list.length <= n ? list.join('; ') : `${list.slice(0, n).join('; ')}; and ${list.length - n} more`;
}

function takeResult(m) {
  const b = takes.get(m.id);
  if (!b) return;
  takes.delete(m.id);
  b.results.push({ ...b.byId.get(m.id), ok: m.ok, err: m.err, entry: m.entry, already: m.already || [] });
  if (b.results.length < b.byId.size) return;
  const undo = b.verb === 'undo';
  const dsts = new Set(b.results.map((r) => r.dst.n)).size;
  const failed = b.results.filter((r) => !r.ok);
  if (!failed.length) {
    const many = dsts > 1 ? ` ${dsts} destinations` : '';
    const title = undo ? `Undone${many && ' on' + many}` : b.verb === 'putback' ? 'Put back' : `Success${many && ':' + many}`;
    showResult('ok', title, capped(b.results.map((r) => routeText(r, r.already))));
    return;
  }
  // The banner follows the failed takes' entries, so it shrinks as they are confirmed late or
  // retried (refreshResult).
  const track = { verb: b.verb, total: b.results.length, items: failed.map((r) => ({ entry: r.entry, dst: r.dst, err: r.err })) };
  const banner = failureBanner(track);
  track.key = banner.key;
  showResult(banner.kind, banner.title, banner.text, track);
}

// Levels grouped by outcome ("levels 0–16 no answer"), at most two groups; entries without
// outcomes give a clipped err.
function outcomeText(e, err) {
  const why = e ? failureGroups(e) : [];
  if (why.length) {
    const shown = why.slice(0, 2).map((g) => `level${g.levels.length === 1 ? '' : 's'} ${levelText(g.levels)} ${g.text}`);
    return shown.join('; ') + (why.length > 2 ? '; …' : '');
  }
  const t = e?.err || err || 'failed';
  return t.length > 90 ? `${t.slice(0, 87)}…` : t;
}

// One line per distinct outcome. A take is resolved after its entry is ok (confirmed late) or a
// retry went through; with none left the banner turns into a confirmation.
export function failureBanner(t, hist = S.hist) {
  const byId = new Map(hist.map((e) => [e.id, e]));
  const retried = new Set(hist.filter((e) => e.ok && e.retry != null).map((e) => e.retry));
  const open = t.items.filter((it) => {
    const e = byId.get(it.entry);
    return !(e && (e.ok || retried.has(e.id)));
  });
  const what = t.verb === 'undo' ? 'Undo' : 'Take';
  if (!open.length) {
    const n = t.items.length;
    const title = `${what} resolved: ${n === 1 ? 'the take' : `all ${n} takes`} confirmed`;
    return { kind: 'ok', title, key: title, text: 'Late Result, or a retry went through.' };
  }
  const lines = new Map();
  for (const it of open) {
    const k = outcomeText(byId.get(it.entry), it.err);
    if (!lines.has(k)) lines.set(k, []);
    lines.get(k).push(it.dst.label);
  }
  const rows = [...lines].map(([k, dsts]) => `${dsts.slice(0, 3).join(', ')}${dsts.length > 3 ? ` +${dsts.length - 3}` : ''}: ${k}`);
  const shown = rows.slice(0, 3);
  if (rows.length > 3) shown.push(`and ${rows.length - 3} more; see History`);
  const partly = open.length < t.total;
  const count = t.total > 1 ? ` (${open.length} of ${t.total}${open.length < t.items.length ? ' still' : ''})` : '';
  const title = `${what} ${partly ? 'partly failed' : 'failed'}${count}`;
  return { kind: 'error', title, key: [title, ...shown].join('\n'), text: () => el('div', null, ...shown.map((l) => el('div', null, l))) };
}

function refreshResult() {
  const r = S.result;
  if (!r?.track) return;
  const next = failureBanner(r.track);
  if (next.key === r.track.key) return;
  r.track.key = next.key;
  setResult({ kind: next.kind, title: next.title, text: next.text, track: r.track });
}
on('history', refreshResult);

// prot: each protected item's protection, keyed by the item itself, so copies of the plan must
// stay shallow. Blocks only when every destination is protected; otherwise those are refused
// and the rest go.
export function planBlocked(plan) {
  const prot = new Map();
  const down = notLinked(plan);
  if (down != null) {
    const text = down ? `Sode is not connected to router ${routerData(down).prefix}.` : 'Sode is not connected to the router.';
    return { block: { kind: 'error', title: 'Not connected', text }, prot };
  }
  if (plan.protect) return { block: null, prot };
  const outside = planOutside(S.myRights, plan, off(), levelLabel, rightsRouter);
  if (outside) return { block: { kind: 'error', title: 'Outside your Rights', text: outside }, prot };
  const live = plan.items.filter((i) => i.groups.length);
  // A take sends each route to the router of its destination, with the source number of that router.
  const across = live.find((i) => i.groups.some((g) => routerOfItem(g.src) !== routerOfItem(i.dst)));
  if (across) {
    return { block: { kind: 'error', title: 'Route across routers', text: `${across.dst.label}: a route across routers needs a tie line.` }, prot };
  }
  for (const i of live) {
    // The page has the Protects of the first router only; the server refuses a protected destination on any router.
    if (routerOfItem(i.dst)) continue;
    const p = holdsAt(
      i.dst.n,
      i.groups.flatMap((g) => g.levels),
    );
    if (p.levels.length) prot.set(i, p);
  }
  const hit = live.filter((i) => prot.has(i));
  if (!hit.length || hit.length < live.length) return { block: null, prot };
  if (hit.length === 1) {
    const [i] = hit;
    const p = prot.get(i);
    return {
      block: {
        kind: 'error',
        title: 'Destination protected',
        text: `${i.dst.label}: ${protectedText(p)}.`,
      },
      prot,
    };
  }
  return {
    block: { kind: 'error', title: 'Destinations protected', text: `All ${hit.length} destinations are protected: ${hit.map((i) => i.dst.label).join(', ')}.` },
    prot,
  };
}

// notLinked gives the router of the plan that has no link ('' for the first router), or null when every one has.
function notLinked(plan) {
  if (!S.ws || S.wsState !== 'up') return '';
  const uids = plan.items.length ? new Set(plan.items.map((i) => routerOfItem(i.dst))) : new Set(['']);
  for (const uid of uids) if (routerData(uid).link !== 'up') return uid;
  return null;
}

function planQuestion(plan) {
  const to = destPart(plan.items.map((i) => i.dst));
  const uniq = (its) => [...new Map(its.map((it) => [it.n, it])).values()];
  if (plan.at) {
    return el('div', null, 'Go back to ', strong(labelNode(srcItem(plan.at.src, plan.at.srcName))), ' on ', ...to, `, as at ${fmtClock(plan.at.time)}?`);
  }
  const stack = plan.items.length === 1 && plan.items[0].undo?.chain;
  if (stack) {
    return el('div', null, `Undo ${stack.length} changes on `, ...to, `, back to before ${fmtClock(stack[stack.length - 1].time)}?`);
  }
  if (plan.verb === 'undo') {
    const srcs = uniq(plan.items.map((i) => srcItem(i.undo.src, i.undo.srcName)));
    return el('div', null, 'Undo the take of ', srcs.length === 1 ? strong(labelNode(srcs[0])) : `${srcs.length} sources`, ' to ', ...to, '?');
  }
  const srcs = uniq(plan.items.flatMap((i) => i.groups.map((g) => g.src)));
  // A mixed plan names the destinations in the order mixedParts shows them: the boxes' destinations first.
  const boxed = (i) => i.groups.some((g) => !g.staged);
  const order = plan.mixed ? [...plan.items.filter(boxed), ...plan.items.filter((i) => !boxed(i))] : plan.items;
  return takeQuestion(
    VERBS[plan.verb].ask,
    srcs,
    order.map((i) => i.dst),
  );
}

// Levels an undo would overwrite besides the take's own: those restored where dst no longer has
// the take's source. Levels the take never confirmed cannot tell, so only the rest are checked;
// an entry without recorded failures checks none.
export function changedSince(item) {
  const e = item.undo;
  if (!e || (!e.ok && !e.failed?.length)) return [];
  const uid = routerOfItem(item.dst);
  const t = routerData(uid).tally.get(e.dst);
  const failed = new Set(e.ok ? [] : e.failed);
  const levels = item.groups.flatMap((g) => g.levels).filter((u) => !failed.has(u) && t?.has(u) && t.get(u) !== e.src);
  return bySource(
    e.dst,
    levels.sort((a, b) => a - b),
    uid,
  );
}

// A non-undo item that changes nothing: dst already has the source on every level (the router
// sends no reply).
export function alreadyThere(item) {
  if (item.undo) return false;
  const t = routerData(routerOfItem(item.dst)).tally.get(item.dst.n);
  return !!t && item.groups.length > 0 && item.groups.every((g) => g.levels.length > 0 && g.levels.every((u) => t.get(u) === g.src.n));
}

function itemSummary(item, boxed, prot, since) {
  const all = item.groups.flatMap((g) => g.levels).sort((a, b) => a - b);
  const uid = routerOfItem(item.dst);
  const before = bySource(item.dst.n, all, uid).map((r) => srcRow(r.levels, tallyText(r.src, uid), null, null, uid));
  return [
    ...item.groups.map((g) => routeRow(g.levels, labelNode(g.src), labelNode(item.dst), uid)),
    ...(since ? changedSince(item) : []).map((r) =>
      el(
        'div',
        'sode__route',
        levelBar(r.levels, uid),
        el('span', 'pb-badge pb-badge--warn', 'Changed since'),
        `now ${tallyText(r.src, uid)}; undo will replace it`,
      ),
    ),
    alreadyThere(item) ? el('div', 'sode__route', el('span', 'pb-badge', 'Already there')) : null,
    prot ? el('div', 'sode__route', levelBar(prot.levels), ...prot.holds.map(holdBadge), 'will be refused') : null,
    item.untouched?.length
      ? el(
          'div',
          'sode__hist-note',
          `Level${item.untouched.length === 1 ? '' : 's'} ${levelText(item.untouched)}: the take did not change ${item.untouched.length === 1 ? 'it' : 'them'}, left as is.`,
        )
      : null,
    item.skipped.length
      ? el('div', 'sode__hist-note', `Level${item.skipped.length === 1 ? '' : 's'} ${levelText(item.skipped)}: nothing was routed before, left as is.`)
      : null,
    ...(!before.length
      ? []
      : boxed
        ? [el('div', 'sode__plan-before', el('div', 'sode__label', 'Replacing'), ...before)]
        : [el('div', 'sode__label', 'Replacing'), ...before]),
  ].filter(Boolean);
}

export function planSummary(plan, { prot = new Map(), spaceHint = false } = {}) {
  if (plan.protect) return protectSummary(plan);
  // Restore goes back past newer changes on purpose; other undos warn.
  const since = plan.verb === 'undo' && !plan.at;
  const head = [
    plan.title ? el('div', 'sode__label', plan.title) : null,
    plan.warnings?.length
      ? el(
          'div',
          'pb-alert pb-alert--warn',
          el('div', null, el('div', 'pb-alert__title', `${plan.warnings.length} left out`), ...plan.warnings.map((l) => el('div', null, l))),
        )
      : null,
  ];
  if (plan.mixed) return [...head, planQuestion(plan), ...mixedParts(plan, prot)].filter(Boolean);
  if (plan.items.length === 1)
    return [...head, planQuestion(plan), ...itemSummary(plan.items[0], false, prot.get(plan.items[0]), since), extraBox(plan, spaceHint)].filter(Boolean);
  return [
    ...head,
    planQuestion(plan),
    ...plan.items.map((i, n) => planItemBox(i, `${n + 1}/${plan.items.length}`, prot.get(i), since)),
    extraBox(plan, spaceHint),
  ].filter(Boolean);
}

function planItemBox(item, count, prot, since, badge = null) {
  return el('section', 'sode__plan-item', planItemHead(count, item.dst, badge), ...itemSummary(item, true, prot, since));
}

// One Take with staged routes and the patch boxes' route (gridPlan, mixed). The boxes' route shows first, under its own
// label. Then each staged route shows with a Staged badge. A destination in both shows in both parts, each with what it
// replaces on its own levels. Its protected levels show once, in the first part.
function mixedParts(plan, prot) {
  const part = (staged) => plan.items.map((i) => ({ i, staged, groups: i.groups.filter((g) => !!g.staged === staged) })).filter((x) => x.groups.length);
  const box = part(false);
  const staged = part(true);
  const total = box.length + staged.length;
  const shown = new Set();
  const itemBox = (x, n) => {
    const p = shown.has(x.i) ? null : prot.get(x.i);
    shown.add(x.i);
    const badge = x.staged ? el('span', 'pb-badge sode__plan-staged', 'Staged') : null;
    return planItemBox({ ...x.i, groups: x.groups }, `${n + 1}/${total}`, p, false, badge);
  };
  return [
    box.length ? el('div', 'sode__label', 'Patch boxes') : null,
    ...box.map((x, n) => itemBox(x, n)),
    staged.length ? el('div', 'sode__label', staged.flatMap((x) => x.groups).length === 1 ? 'Staged route' : 'Staged routes') : null,
    ...staged.map((x, n) => itemBox(x, box.length + n)),
  ];
}

function extraBox(plan, spaceHint) {
  const x = plan.extra;
  if (!x) return null;
  const n = x.items.length;
  const names = x.items.map((i) => i.dst.label);
  const c = el('input', 'pb-check__input');
  c.type = 'checkbox';
  c.checked = x.on;
  c.addEventListener('change', () => {
    x.on = c.checked;
  });
  return el(
    'label',
    'pb-check sode__plan-extra',
    c,
    el(
      'span',
      'pb-check__label',
      `Also undo ${n} External follow ${n === 1 ? 'entry' : 'entries'}: ${names.slice(0, 3).join(', ')}${n > 3 ? ', …' : ''}${spaceHint ? ' (Space)' : ''}`,
    ),
  );
}

// This browser's last patch as numbers, so names stay current. Null until it makes one: History
// holds other people's takes and must not fill a box. Stored per browser (every tab and user
// shares it); memory answers only when storage is blocked or this tab's last write did not land.
const LAST_KEY = 'sode-last-take';
let lastPatch = null;
let lastWriteFailed = false;

// Another tab's take: its Last (and the Next built on it) shows here too.
globalThis.addEventListener?.('storage', (e) => {
  if (e.key === LAST_KEY || e.key === null) emit('last');
});

const isNum = (v) => Number.isInteger(v) && v >= 0;
const isPair = (v) => Array.isArray(v) && isNum(v[0]) && typeof v[1] === 'string';
const validLast = (t) =>
  !!t &&
  isPair(t.src) &&
  Array.isArray(t.dsts) &&
  t.dsts.length > 0 &&
  t.dsts.every(isPair) &&
  Array.isArray(t.levels) &&
  t.levels.every(isNum) &&
  (t.router === undefined || typeof t.router === 'string');

// A redo counts when all its routes share one source and one set of levels; Retry, Preset and
// undo never set Last. Null means leave Last as it is. lastOf is the patch inside a larger take that
// sets Last (the Grid popup's One Take, gridpopup.js).
export function lastFromPlan(p) {
  if (p.lastOf) return lastFromPlan(p.lastOf);
  if ((p.verb !== 'patch' && p.verb !== 'redo') || p.ofUndo) return null;
  const gs = p.items.flatMap((i) => i.groups.map((g) => ({ dst: i.dst, g })));
  if (!gs.length) return null;
  const { src, levels } = gs[0].g;
  const sorted = (l) => [...l].sort((x, y) => x - y);
  const lv = sorted(levels);
  if (p.verb === 'redo' && !gs.every(({ g }) => g.src.n === src.n && sameList(sorted(g.levels), lv))) return null;
  const last = { src: [src.n, src.name], dsts: gs.map(({ dst }) => [dst.n, dst.name]), levels: lv };
  // A take on a router after the first names it.
  const uid = routerOfItem(src);
  return uid ? { ...last, router: uid } : last;
}

// A stored value that fails to parse is no Last, not a cue to fall back to memory.
function readLast() {
  let raw;
  try {
    raw = localStorage.getItem(LAST_KEY);
  } catch {
    return lastPatch;
  }
  if (raw == null) return lastWriteFailed ? lastPatch : null;
  try {
    const t = JSON.parse(raw);
    return validLast(t) ? t : null;
  } catch {
    return null;
  }
}

export function lastTake() {
  const t = readLast();
  const uid = t?.router || '';
  // A router that this page no longer has gives no Last.
  if (!t || routerOfItem({ router: uid }) !== uid) return null;
  const rt = routerData(uid);
  if (!rt.srcBy.has(t.src[0]) || !t.dsts.every(([n]) => rt.dstBy.has(n))) return null;
  return {
    src: srcItem(...t.src, uid),
    dsts: t.dsts.map(([n, name]) => dstItem(n, name, uid)),
    levels: t.levels.filter((u) => S.levelBy.has(u)),
  };
}

// Last take with source and every destination one number higher (Cam 1 → Rec 1 + Rec 5 then Cam
// 2 → Rec 2 + Rec 6). A side is null/empty when any number would run past the router's list.
// Levels are not carried. Repeats: while a box still holds what the last Next fill put there
// (run), the next step starts from that. A take, Clear or hand pick ends the run by itself.
let run = { src: null, dsts: null };

export function nextTake(state) {
  const t = lastTake();
  if (!t) return null;
  const src = run.src && state?.src?.n === run.src.n ? run.src : t.src;
  const base = run.dsts && state && run.dsts.every((d) => state.dsts.some((x) => x.n === d.n)) ? run.dsts : t.dsts;
  const rt = routerData(routerOfItem(t.src));
  const dsts = base.map((d) => nextOf(d, rt.dstBy));
  return { src: nextOf(src, rt.srcBy), dsts: dsts.every(Boolean) ? dsts : [], levels: [] };
}

export function usedNext(k, t) {
  if (k === 'src') run.src = t.src;
  else if (k === 'dst') run.dsts = t.dsts;
}

export function resetNext() {
  run = { src: null, dsts: null };
}

// The name search nextAfter (boxlist.js) uses for the Next match behavior setting: null for Next number. The Next chip
// and the next add both take it, so they use the one setting.
export function nextFind(items) {
  return pref('nextMode') === 'fuzzy' ? (q) => search(items, q, items.length) : null;
}

// The next number, or with Next match of the name the next router-order match on the name's text.
function nextOf(it, by) {
  const all = [...by.values()];
  return nextAfter(it, all, { find: nextFind(all) });
}

const sameList = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);

// false when the box already has it. For levels, also false when they are the default.
export function lastChip(t, state) {
  if (!t) return { src: false, dst: false, lvl: false };
  return {
    src: t.src && !(state.src && state.src.n === t.src.n) && [labelNode(t.src)],
    dst: !t.dsts.every((d) => state.dsts.some((x) => x.n === d.n)) && [labelNode(t.dsts[t.dsts.length - 1]), t.dsts.length > 1 ? ` +${t.dsts.length - 1}` : ''],
    lvl: t.levels.length > 0 && !sameList(state.levels, t.levels) && !sameList(t.levels, parseLevels('') || []) && [levelText(t.levels)],
  };
}

export function sendTakes(p) {
  if (p.protect) return sendProtect(p);
  const take = { verb: p.verb, byId: new Map(), results: [] };
  const items = [];
  for (const it of [...p.items, ...(p.extra?.on ? p.extra.items : [])]) {
    for (const g of it.groups) {
      const id = ++takeSeq;
      const msg = { id, src: g.src.n, dst: it.dst.n, levels: g.levels };
      if (routerOfItem(it.dst)) msg.router = routerOfItem(it.dst);
      if (it.undo) msg.undo = it.undo.id;
      if (it.putBack) msg.putBack = true;
      if (it.retry) msg.retry = it.retry.id;
      items.push(msg);
      take.byId.set(id, { dst: it.dst, g });
    }
  }
  const out = { t: 'take', items };
  if (p.verb === 'preset' && p.preset) Object.assign(out, { preset: p.preset, presetId: p.presetId });
  if (!wsSend(out)) {
    showResult('error', 'Not sent', `${LINK_DOWN} Try again.`);
    return false;
  }
  for (const id of take.byId.keys()) takes.set(id, take);
  // The Grid popup drops its staged routes only once they went out.
  p.onSent?.();
  const last = lastFromPlan(p);
  if (last) {
    lastPatch = last;
    // A full storage keeps the older value, which would outrank memory.
    lastWriteFailed = !writeJSON(LAST_KEY, last);
    if (lastWriteFailed) writeStr(LAST_KEY, null);
  }
  const list = [...take.byId.values()];
  const dsts = new Set(list.map((t) => t.dst.n)).size;
  const text =
    dsts > 1 && new Set(list.map((t) => t.g.src.n)).size === 1
      ? `${list[0].g.src.label} → ${dsts} destinations (${routeLevels(list[0].g.levels, routerOfItem(list[0].dst))})`
      : capped(list.map((t) => routeText(t)));
  showResult('info', VERBS[p.verb].busy, text);
  return true;
}

// The signed-in user's own Rights (`myRights`, never in the hello). Null means nothing is greyed out;
// the server refuses a take outside them whatever this page thinks.
export const applyMyRights = (r) => {
  S.myRights = r || null;
  emit('myRights');
};
// rightsRouter: the UID that a Rights check compares for a route end; Rights store the first router's UID too.
export const rightsRouter = (end) => routerData(routerOfItem(end)).uid;
export const rightsOff = (kind, end) => !allowed(S.myRights, kind, end.n, rightsRouter(end));
// editorrights.js owns the message on every page, so the Palette reads the same Rights.
onMyRights(applyMyRights);

onMsg('takeResult', takeResult);
on('wsclose', () => {
  if (!takes.size) return;
  takes.clear();
  showResult('warn', 'Connection lost', 'Take sent, Result unknown. Check History.');
});
