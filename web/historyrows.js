// History rows, for the desktop list and the phone's swipe rows. Pure: wording and plans only,
// each layout builds its own DOM from these.

import { S, srcItem, dstItem, routerOfItem } from './core.js';
import { hopsVia, alsoNote } from './viarules.js';
import { undoPlan, againPlan, groupUndoPlan, lastTake, nextTake, usedNext } from './plans.js';
import { groupPartial, groupIds } from './history.js';
import { UNDO_TIP_ALL } from './widgets.js';
import { PROTECT_ENTRY, PROTECT_GROUP } from './protects.js';
import { protectGroup } from './protectrules.js';

const noun = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`;
const againWord = (again) => (again.verb === 'retry' ? 'Retry' : 'Redo');

// The source a row names. The entry of a tie take is the last hop, so its src is the end of the tie line. origin
// is the source asked for.
const srcOf = (e) => (e.origin ? srcItem(e.origin.n, e.origin.name, e.origin.router) : srcItem(e.src, e.srcName, e.router));

// A tie take that failed after a hop changed its tie line (docs/tielines-design.md, "Order and failures"): "TIE 3
// changed, B.42 not". The hops in e.hops with result connected changed their tie lines. A hop with late changed its
// tie line after the take gave up, so the hops after it were not sent: "TIE 3 changed late".
const changedHops = (e) => (!e.ok && e.hops ? e.hops.filter((h) => (h.result === 'connected' || h.late) && h.entry != null) : []);
// The words of an External on the upstream end of a tie line: what it also changes.
const alsoOf = (e) => alsoNote(e, (a) => dstItem(a.n, a.name || '', routerOfItem(a)).label);

const tieNames = (hs) => hs.map((h) => h.tieName || h.tie).join(', ');
function partialNote(e) {
  const hs = changedHops(e);
  if (!hs.length) return null;
  const inTime = hs.filter((h) => !h.late);
  const late = hs.filter((h) => h.late);
  const parts = [inTime.length && `${tieNames(inTime)} changed`, late.length && `${tieNames(late)} changed late`].filter(Boolean);
  return inTime.length ? `${parts.join(', ')}, ${dstItem(e.dst, e.dstName, e.router).label} not` : parts[0];
}

// Put back: one for each hop entry of the Group that changed a tie line for a route that failed. Each is
// {plan, label, text}. The plan is the Undo of the hop entry, tagged putBack. The server sends it only while the tie
// line still carries what the take set. A hop that an Undo already put back offers none.
export function putBacks(entries, hist = S.hist) {
  const ids = new Set(entries.flatMap((e) => changedHops(e).map((h) => h.entry)));
  const done = new Set(hist.filter((x) => x.ok && x.undo != null).map((x) => x.undo));
  const out = [];
  for (const hop of entries) {
    if (!hop.tie || !ids.has(hop.id) || done.has(hop.id)) continue;
    const name = entries.flatMap((e) => e.hops || []).find((h) => h.entry === hop.id)?.tieName || hop.tie;
    const srcs = new Set(hop.prev.filter(([u]) => hop.levels.includes(u)).map(([, s]) => s));
    if (srcs.has(-1) && srcs.size === 1) continue;
    const to = srcs.size === 1 ? ` to ${srcItem([...srcs][0], '', hop.router).label}` : '';
    const plan = undoPlan(hop);
    const label = `Put ${name} back${to}`;
    out.push({
      plan: { ...plan, verb: 'putback', title: label, items: plan.items.map((i) => ({ ...i, putBack: true })) },
      label,
      text: `Put ${name} back`,
    });
  }
  return out;
}

// A take entry. Each descriptor has, for an entry ({e}) or a Group ({f}):
// - route(x): `{lead, src, via, dst, many, notes}`. lead: a node that stands where the source is (a Protect chip), else
//   null. `via`: the Via of a tie take ("Via TIE 3"), else ''.
//   src and dst: an item, or words for many ("3 sources"). many: a Group's destinations, else null. notes: a Group's
//   words in brackets.
// - disc(e, before) and detail(e, dst, before) (entries): the closed disclosure's word and the open detail's heading;
//   before is beforeGroups(e).
// - noun (Groups): what the disclosure counts.
// - actions(x): undo {plan, label, text, tip}, again {plan, label} and putBack (a list, see putBacks). tip is undefined unless the row has its own
//   wording (the buttons then use their default).
// - empty(x): what Undo says when there is nothing to undo.
const TAKE = {
  route: (e) => ({
    lead: null,
    src: srcOf(e),
    via: hopsVia(e),
    dst: dstItem(e.dst, e.dstName, e.router),
    many: null,
    notes: [partialNote(e), alsoOf(e)].filter(Boolean),
  }),
  disc: () => 'Before',
  detail: (e, dst) => `Before #${e.id}, on ${dst.label}`,
  actions(e) {
    const src = srcOf(e),
      dst = dstItem(e.dst, e.dstName, e.router);
    const again = againPlan({ e });
    const undo = e.chain
      ? {
          plan: undoPlan(e),
          label: `Undo ${e.chain.length} changes on ${dst.label}`,
          text: `Undo ${e.chain.length}`,
          tip: 'Restores the source from before these changes',
        }
      : { plan: undoPlan(e), label: `Undo ${src.label} to ${dst.label}`, text: 'Undo', tip: undefined };
    return { undo, again: { plan: again, label: `${againWord(again)} ${src.label} to ${dst.label}` }, putBack: [] };
  },
  empty: (e) => `Nothing was routed before #${e.id}.`,
};

// A Group of takes: a take that sent several routes.
const GROUP = {
  route(f) {
    // The hop entries of a tie take (tie) are part of the routes asked for, so the row names only those routes. The
    // Protect of a tie line after the take (tieprotect.go) is no route either.
    const asked = f.entries.filter((e) => !e.tie && !e.protect);
    const es = asked.length ? asked : f.entries;
    const srcs = new Set(es.map((e) => (e.origin ? `${e.origin.router} ${e.origin.n}` : e.src)));
    const dsts = new Map(es.map((e) => [e.dst, e.dstName]));
    const kind = f.entries.every((e) => e.undo != null) ? 'undo' : f.entries.every((e) => e.retry != null) ? 'retry' : '';
    const nFollow = f.entries.reduce((n, e) => n + (e.follows?.length || 0), 0);
    const preset = f.entries.find((e) => e.preset)?.preset;
    const notes = [
      preset && `Preset: ${preset}`,
      kind,
      groupPartial(f) && `${f.shown.length} of ${f.entries.length} shown`,
      nFollow && `+${nFollow} External follow ${nFollow === 1 ? 'entry' : 'entries'}`,
      ...asked.map(partialNote),
      ...asked.map(alsoOf),
    ];
    const vias = new Set(es.map(hopsVia));
    return {
      lead: null,
      src: srcs.size === 1 ? srcOf(es[0]) : `${srcs.size} sources`,
      via: srcs.size === 1 && vias.size === 1 ? [...vias][0] : '',
      dst: dsts.size === 1 ? dstItem(es[0].dst, es[0].dstName) : `${dsts.size} destinations`,
      many: dsts.size > 1 ? [...dsts].map(([n, name]) => dstItem(n, name)) : null,
      notes: notes.filter(Boolean),
    };
  },
  noun: 'takes',
  actions(f) {
    const again = againPlan({ f });
    return {
      undo: {
        plan: groupUndoPlan(f),
        label: `Undo all ${f.entries.length} routes of ${groupIds(f)}`,
        text: groupPartial(f) ? `Undo all ${f.entries.length}` : 'Undo all',
        tip: UNDO_TIP_ALL,
      },
      again: { plan: again, label: `${againWord(again)} ${noun(again.items.length, 'route')} of ${groupIds(f)}` },
      putBack: putBacks(f.entries),
    };
  },
  empty: (f) => `Nothing was routed before ${groupIds(f)}.`,
};

// The descriptor of a History row ({e} or {f}): the one place that tells the kinds apart.
export function rowView(row) {
  if (row.f) return protectGroup(row.f) ? PROTECT_GROUP : GROUP;
  return row.e.protect ? PROTECT_ENTRY : TAKE;
}

export const rowActions = (row) => rowView(row).actions(row.f || row.e);

// Aria label of a Group's disclosure: "Show the 3 takes", "Hide the 2 followers".
export const disclosureAria = (open, n, what) => `${open ? 'Hide' : 'Show'} the ${n} ${what}`;

// The External follow header of a Group: the disclosure's count text, aria label, and the id of its kids.
export function followersHead(f, open, idPrefix) {
  return {
    count: `${f.followers.length}`,
    aria: disclosureAria(open, f.followers.length, 'External follow entries'),
    kidsId: `${idPrefix}-followers-${f.group}`,
  };
}

// The Last / Next fill of one box (k: src, dst or lvl). null: nothing to fill for k; {none: true}:
// no earlier take at all; else {t}, with a Next fill remembered so a repeat goes on. state: the boxes.
export function lastFill(k, next, state) {
  const t = next ? nextTake(state) : lastTake();
  if (!t) return { none: true };
  if (k === 'src' && !t.src) return null;
  if (k === 'dst' && !t.dsts.length) return null;
  if (next && k !== 'lvl') usedNext(k, t);
  return { t };
}
