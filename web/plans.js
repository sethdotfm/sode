// Plans (what a confirm shows and sends), take results and their banners.
// A plan: { verb, items: [{ dst, groups: [{src, levels}], skipped, undo, retry }] }

import { el, S, on, onMsg, emit, wsSend, wsFreshNow, wsReset, linked, protectedOn, protectedLabel, srcItem, dstItem, labelNode, tallyText } from './core.js';
import { levelText, levelBar, parseLevels } from './levelview.js';
import { bySource, srcRow, routeRow } from './routes.js';
import { fmtClock } from './device.js';
import { failureGroups } from './history.js';

let takeSeq = 0;
const takes = new Map(); // take id → batch

// ---- plans: { verb, items: [{ dst, groups: [{src, levels}], skipped, undo, retry }] }
// One item per destination. undo/retry are the history entries an item
// undoes or retries. Protection isn't kept here: planBlocked returns it.

// Levels of a failed take that the router answered for, so it is known the
// take didn't change them: still on their old route, or on someone else's.
// Undo leaves them alone (restoring would overwrite a route that isn't the
// take's). Levels with no answer stay in: they may have switched.
// Late levels did change with the take, whatever `now` says.
const untouchedLevels = (e) => new Set(e.ok ? [] : (e.now || []).map(([u]) => u).filter((u) => !e.late?.includes(u)));

// Groups an entry's previous routing into restorable takes.
function undoItem(e) {
  const groups = new Map();
  const skipped = [];
  const untouched = untouchedLevels(e);
  for (const [u, s] of e.prev) {
    if (!S.levelBy.has(u) || untouched.has(u)) continue;
    if (s < 0) { skipped.push(u); continue; }
    if (!groups.has(s)) groups.set(s, []);
    groups.get(s).push(u);
  }
  const known = new Set(e.prev.map(([u]) => u));
  for (const u of e.levels) if (!known.has(u) && !untouched.has(u)) skipped.push(u);
  return {
    dst: dstItem(e.dst, e.dstName),
    groups: [...groups].map(([s, levels]) => ({ src: srcItem(s), levels: levels.sort((a, b) => a - b) })),
    skipped: skipped.sort((a, b) => a - b),
    untouched: [...untouched].filter((u) => S.levelBy.has(u)).sort((a, b) => a - b),
    undo: e, retry: null,
  };
}

// A failed take, tried again as a fresh patch of the same route.
const retryItem = (e) => ({
  dst: dstItem(e.dst, e.dstName),
  groups: [{ src: srcItem(e.src, e.srcName), levels: e.levels.filter((u) => S.levelBy.has(u)).sort((a, b) => a - b) }],
  skipped: [], untouched: [], undo: null, retry: e,
});

export const patchPlan = (src, dsts, levels) => ({
  verb: 'patch',
  items: dsts.map((dst) => ({ dst, groups: [{ src, levels }], skipped: [], untouched: [], undo: null, retry: null })),
});
// Follows (changes the router made after a take, see histRows) are left
// out of an undo unless the confirm's box is ticked: extra.on. extra is
// shared by copies of the plan, so the box works on whichever is sent.
// A destination followed more than once (a merged row) goes back to before
// the oldest follow: one undo each, not several racing.
function withFollows(plan, follows) {
  const oldest = new Map();
  for (const f of follows) if (!oldest.has(f.dst) || f.id < oldest.get(f.dst).id) oldest.set(f.dst, f);
  return oldest.size ? { ...plan, extra: { items: [...oldest.values()].map(undoItem), on: false } } : plan;
}
// Back to how a stacked row's destination was at one of its changes,
// at: by undoing the change that came after it, then.
export const restorePlan = (then, at) => ({ verb: 'undo', items: [undoItem(then)], at });
export const undoPlan = (e) => withFollows({ verb: 'undo', items: [undoItem(e)] }, e.follows || []);
export const retryPlan = (e) => ({ verb: 'retry', items: [retryItem(e)] });
export const folderUndoPlan = (f) => withFollows({ verb: 'undo', items: f.entries.map(undoItem) }, f.entries.flatMap((e) => e.follows || []));
export const folderRetryPlan = (f) => ({ verb: 'retry', items: f.entries.filter((e) => !e.ok).map(retryItem) });
// Undo for a history row: a single entry or a whole folder.
export const rowUndoPlan = (row) => (row.f ? folderUndoPlan(row.f) : undoPlan(row.e));

// 'undo', 'retry' or 'take' (a 'patch' plan), for wording confirms.
export const planVerb = (plan) => (plan.verb === 'patch' ? 'take' : plan.verb);
// True when a plan has nothing to send (an undo with nothing routed before).
export const planEmpty = (plan) => !plan.items.some((i) => i.groups.length);
// ---- results

// text: a string, or a function building nodes (each layout renders its own).
export function showResult(kind, title, text, track = null) {
  S.result = { kind, title, text, track };
  emit('result', S.result);
}

export function resultNode(r) {
  return el('div', `pb-alert pb-alert--${r.kind}`, el('div', null, el('div', 'pb-alert__title', r.title), typeof r.text === 'function' ? r.text() : r.text));
}

// "A → B (levels 1–3; 2 was already on A)" for one take.
function takeText(t, already = []) {
  return `${t.g.src.label} → ${t.dst.label} (levels ${levelText(t.g.levels)}` +
    (already.length ? `; ${levelText(already)} ${already.length === 1 ? 'was' : 'were'} already on ${t.g.src.label}` : '') + ')';
}

// Joins the first few of list, then "and N more".
function capped(list, n = 3) {
  return list.length <= n ? list.join('; ') : `${list.slice(0, n).join('; ')}; and ${list.length - n} more`;
}

// A batch is the takes from one confirm. It reports once, after every
// take has answered.
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
    const title = undo ? `Undone${many && ' on' + many}` : `Success${many && ':' + many}`;
    showResult('ok', title, capped(b.results.map((r) => takeText(r, r.already))));
    return;
  }
  // The banner follows the entries of the failed takes, so it shrinks as
  // they're confirmed late or retried (refreshResult), instead of going stale.
  const track = { verb: b.verb, total: b.results.length, items: failed.map((r) => ({ entry: r.entry, dst: r.dst, err: r.err })) };
  const banner = failureBanner(track);
  track.key = banner.key;
  showResult(banner.kind, banner.title, banner.text, track);
}

// What the router said about a failed take, short: its levels grouped by
// outcome ("levels 0–16 no answer"), at most two groups; older entries
// (before outcomes were recorded) and failures with no breakdown give a
// clipped err.
function outcomeText(e, err) {
  const why = e ? failureGroups(e) : [];
  if (why.length) {
    const shown = why.slice(0, 2).map((g) => `level${g.levels.length === 1 ? '' : 's'} ${levelText(g.levels)} ${g.text}`);
    return shown.join('; ') + (why.length > 2 ? '; …' : '');
  }
  const t = e?.err || err || 'failed';
  return t.length > 90 ? `${t.slice(0, 87)}…` : t;
}

// The result banner for failed takes of one confirm, from the history now:
// one line per distinct outcome, naming a few destinations. A take counts as
// resolved once its entry is ok (confirmed late) or a retry of it went
// through; with none left the banner turns into a confirmation.
// t: { verb, total, items: [{ entry, dst, err }] }.
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
    return { kind: 'ok', title, key: title, text: 'The router confirmed after all, or a retry went through.' };
  }
  const lines = new Map();
  for (const it of open) {
    const k = outcomeText(byId.get(it.entry), it.err);
    if (!lines.has(k)) lines.set(k, []);
    lines.get(k).push(it.dst.label);
  }
  const rows = [...lines].map(([k, dsts]) => `${dsts.slice(0, 3).join(', ')}${dsts.length > 3 ? ` +${dsts.length - 3}` : ''}: ${k}`);
  const shown = rows.slice(0, 3);
  if (rows.length > 3) shown.push(`and ${rows.length - 3} more; see Recent`);
  const partly = open.length < t.total;
  const count = t.total > 1 ? ` (${open.length} of ${t.total}${open.length < t.items.length ? ' still' : ''})` : '';
  const title = `${what} ${partly ? 'partly failed' : 'failed'}${count}`;
  return { kind: 'error', title, key: [title, ...shown].join('\n'),
    text: () => el('div', null, ...shown.map((l) => el('div', null, l))) };
}

// Redraws the banner as the failed takes it names get resolved.
function refreshResult() {
  const r = S.result;
  if (!r?.track) return;
  const next = failureBanner(r.track);
  if (next.key === r.track.key) return;
  r.track.key = next.key;
  S.result = { kind: next.kind, title: next.title, text: next.text, track: r.track };
  emit('result', S.result);
}
on('history', refreshResult);

// Whether a plan can be confirmed right now. block: why not, as a result
// banner, or null. prot: each protected item's protection ({levels,
// devices}), keyed by the item itself, so copies of the plan must stay
// shallow. A plan only blocks when every destination is protected;
// otherwise those are refused and the rest go.
export function planBlocked(plan) {
  const prot = new Map();
  if (!linked()) {
    return { block: { kind: 'error', title: 'Not connected', text: 'Sode is not connected to the router.' }, prot };
  }
  const live = plan.items.filter((i) => i.groups.length);
  for (const i of live) {
    const p = protectedOn(i.dst.n, i.groups.flatMap((g) => g.levels));
    if (p.levels.length) prot.set(i, p);
  }
  const hit = live.filter((i) => prot.has(i));
  if (!hit.length || hit.length < live.length) return { block: null, prot };
  if (hit.length === 1) {
    const [i] = hit;
    const p = prot.get(i);
    return {
      block: {
        kind: 'error', title: 'Destination protected',
        text: `${i.dst.label} is protected on levels ${levelText(p.levels)} by device ${p.devices.join(', ')}.`,
      },
      prot,
    };
  }
  return { block: { kind: 'error', title: 'Destinations protected', text: `All ${hit.length} destinations are protected: ${hit.map((i) => i.dst.label).join(', ')}.` }, prot };
}

function planQuestion(plan) {
  const strong = (t) => el('strong', null, t);
  const n = plan.items.length;
  const to = n === 1 ? ['Destination ', strong(labelNode(plan.items[0].dst))] : [strong(`${n} destinations`)];
  const uniq = (its) => [...new Map(its.map((it) => [it.n, it])).values()];
  if (plan.at) {
    return el('div', null, 'Are you sure you want to go back to ', strong(labelNode(srcItem(plan.at.src, plan.at.srcName))),
      ' on ', ...to, `, as at ${fmtClock(plan.at.time)}?`);
  }
  const stack = n === 1 && plan.items[0].undo?.chain;
  if (stack) {
    return el('div', null, `Are you sure you want to undo ${stack.length} changes on `, ...to,
      `, back to before ${fmtClock(stack[stack.length - 1].time)}?`);
  }
  if (plan.verb === 'undo') {
    const srcs = uniq(plan.items.map((i) => srcItem(i.undo.src, i.undo.srcName)));
    return el('div', null, 'Are you sure you want to undo the take of ', srcs.length === 1 ? strong(labelNode(srcs[0])) : `${srcs.length} sources`,
      ' to ', ...to, '?');
  }
  const srcs = uniq(plan.items.flatMap((i) => i.groups.map((g) => g.src)));
  return el('div', null, `Are you sure you want to ${plan.verb === 'retry' ? 'retry taking' : 'take'} `,
    ...(srcs.length === 1 ? ['Source ', strong(labelNode(srcs[0]))] : [strong(`${srcs.length} sources`)]), ' to ', ...to, '?');
}

// What an undo item would overwrite besides the take it undoes: the levels
// it restores where dst no longer has that take's source, because
// something changed them since. [{src, levels}] by what's there now. A
// failed take may have left some levels as they were: those it never
// confirmed (failed) can't tell, so only the rest are checked. An entry
// from before failures were recorded can't say which, so it checks none.
export function changedSince(item) {
  const e = item.undo;
  if (!e || (!e.ok && !e.failed?.length)) return [];
  const t = S.tally.get(e.dst);
  const failed = new Set(e.ok ? [] : e.failed);
  const levels = item.groups.flatMap((g) => g.levels).filter((u) => !failed.has(u) && t?.has(u) && t.get(u) !== e.src);
  return bySource(e.dst, levels.sort((a, b) => a - b));
}

// One destination's part of a confirm: new routes, then what they replace.
// since: warn about changes made after the take being undone.
function itemSummary(item, boxed, prot, since) {
  const all = item.groups.flatMap((g) => g.levels).sort((a, b) => a - b);
  const before = bySource(item.dst.n, all).map((r) => srcRow(r.levels, tallyText(r.src)));
  return [
    ...item.groups.map((g) => routeRow(g.levels, labelNode(g.src), labelNode(item.dst))),
    ...(since ? changedSince(item) : []).map((r) => el('div', 'sode__route', levelBar(r.levels),
      el('span', 'pb-badge pb-badge--warn', 'Changed since'), `now ${tallyText(r.src)}; undo will replace it`)),
    prot
      ? el('div', 'sode__route', levelBar(prot.levels), el('span', 'pb-badge pb-badge--warn', protectedLabel(prot.devices)),
        'will be refused')
      : null,
    item.untouched?.length
      ? el('div', 'sode__hist-note', `Level${item.untouched.length === 1 ? '' : 's'} ${levelText(item.untouched)}: the take didn't change ${item.untouched.length === 1 ? 'it' : 'them'}, left as is.`)
      : null,
    item.skipped.length
      ? el('div', 'sode__hist-note', `Level${item.skipped.length === 1 ? '' : 's'} ${levelText(item.skipped)}: nothing was routed before, left as is.`)
      : null,
    ...(!before.length ? [] : boxed
      ? [el('div', 'sode__plan-before', el('div', 'sode__label', 'Replacing'), ...before)]
      : [el('div', 'sode__label', 'Replacing'), ...before]),
  ].filter(Boolean);
}

// What a confirm shows: the question, then each destination's new routes
// and what they replace. With several destinations each gets its own
// block, headed by its name, so the groups are easy to tell apart.
// prot: from planBlocked. spaceHint: the follows box says Space ticks it.
export function planSummary(plan, { prot = new Map(), spaceHint = false } = {}) {
  // Restore goes back past newer changes on purpose; other undos warn.
  const since = plan.verb === 'undo' && !plan.at;
  if (plan.items.length === 1) return [planQuestion(plan), ...itemSummary(plan.items[0], false, prot.get(plan.items[0]), since), extraBox(plan, spaceHint)].filter(Boolean);
  return [
    planQuestion(plan),
    ...plan.items.map((i, n) => el('section', 'sode__plan-item',
      el('div', 'sode__plan-head', el('span', 'sode__plan-n', `${n + 1}/${plan.items.length}`), el('strong', null, labelNode(i.dst))),
      ...itemSummary(i, true, prot.get(i), since))),
    extraBox(plan, spaceHint),
  ].filter(Boolean);
}

// The box that adds an undo's follows, or null.
function extraBox(plan, spaceHint) {
  const x = plan.extra;
  if (!x) return null;
  const n = x.items.length;
  const names = x.items.map((i) => i.dst.label);
  const c = el('input', 'pb-check__input');
  c.type = 'checkbox';
  c.checked = x.on;
  c.addEventListener('change', () => { x.on = c.checked; });
  return el('label', 'pb-check sode__plan-extra', c, el('span', 'pb-check__label',
    `Also undo ${n} suspected automation${n === 1 ? '' : 's'}: ${names.slice(0, 3).join(', ')}${n > 3 ? ', …' : ''}${spaceHint ? ' (Space)' : ''}`));
}

// This browser's last patch (not an undo or retry) as numbers, so names
// stay current; until it makes one, the newest take in history made in
// Sode. For the "Last:" chips and Ctrl+S/D/F.
let lastPatch = null;

export function lastTake() {
  let t = lastPatch;
  if (!t) {
    const e = S.hist.find((x) => !x.ext && x.undo == null && x.retry == null);
    if (!e) return null;
    const es = e.group && e.groupN > 1 ? S.hist.filter((x) => x.group === e.group).reverse() : [e];
    t = { src: [e.src, e.srcName], dsts: es.map((x) => [x.dst, x.dstName]), levels: e.levels };
  }
  return { src: srcItem(...t.src), dsts: t.dsts.map((d) => dstItem(...d)), levels: t.levels.filter((u) => S.levelBy.has(u)) };
}

const sameList = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);

// What the "Last:" chip of each box shows for take t (from lastTake), or
// false when the box already has it. state: { src: the picked source or
// null, dsts: the destinations already picked, levels: the levels now
// typed }. For levels, also false when they're the default.
export function lastChip(t, state) {
  if (!t) return { src: false, dst: false, lvl: false };
  return {
    src: !(state.src && state.src.n === t.src.n) && [labelNode(t.src)],
    dst: !t.dsts.every((d) => state.dsts.some((x) => x.n === d.n)) &&
      [labelNode(t.dsts[t.dsts.length - 1]), t.dsts.length > 1 ? ` +${t.dsts.length - 1}` : ''],
    lvl: t.levels.length > 0 && !sameList(state.levels, t.levels) && !sameList(t.levels, parseLevels('') || []) && [levelText(t.levels)],
  };
}

// Sends every take in a plan as one batch. False if the connection is gone.
export function sendTakes(p) {
  const batch = { verb: p.verb, byId: new Map(), results: [] };
  const items = [];
  for (const it of [...p.items, ...(p.extra?.on ? p.extra.items : [])]) {
    for (const g of it.groups) {
      const id = ++takeSeq;
      const msg = { id, src: g.src.n, dst: it.dst.n, levels: g.levels };
      if (it.undo) msg.undo = it.undo.id;
      if (it.retry) msg.retry = it.retry.id;
      items.push(msg);
      batch.byId.set(id, { dst: it.dst, g });
    }
  }
  if (!wsFreshNow()) {
    // Nothing has come from the server lately: the socket is probably dead
    // though it says open. Don't send into it; reconnect instead.
    wsReset();
    showResult('error', 'Not sent', 'The patch was NOT sent: the connection to the Sode server went stale. It is reconnecting; try again in a moment.');
    return false;
  }
  if (!wsSend({ t: 'batch', items })) {
    showResult('error', 'Not connected', 'Lost connection to Sode before the take was sent.');
    return false;
  }
  for (const id of batch.byId.keys()) takes.set(id, batch);
  if (p.verb === 'patch') {
    const g = p.items[0].groups[0];
    lastPatch = { src: [g.src.n, g.src.name], dsts: p.items.map((i) => [i.dst.n, i.dst.name]), levels: g.levels };
  }
  const list = [...batch.byId.values()];
  const dsts = new Set(list.map((t) => t.dst.n)).size;
  const text = dsts > 1 && new Set(list.map((t) => t.g.src.n)).size === 1
    ? `${list[0].g.src.label} → ${dsts} destinations (levels ${levelText(list[0].g.levels)})`
    : capped(list.map((t) => takeText(t)));
  showResult('info', p.verb === 'undo' ? 'Undoing…' : 'Taking…', text);
  return true;
}

onMsg('takeResult', takeResult);
on('wsclose', () => {
  if (!takes.size) return;
  // Replies to a dead socket are lost; the outcome is in history.
  takes.clear();
  showResult('warn', 'Connection lost', 'The take was sent but its result is unknown. Check Recent once reconnected.');
});
