// History rows, for the desktop list and the phone's swipe rows. Pure: wording and plans only,
// each layout builds its own DOM from these.

import { srcItem, dstItem } from './core.js';
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

// A take entry. Each descriptor has, for an entry ({e}) or a Group ({f}):
// - route(x): {lead, src, dst, many, notes}. lead: a node that stands where the source is (a Protect chip), else null.
//   src and dst: an item, or words for many ("3 sources"). many: a Group's destinations, else null. notes: a Group's
//   words in brackets.
// - disc(e, before) and detail(e, dst, before) (entries): the closed disclosure's word and the open detail's heading;
//   before is beforeGroups(e).
// - noun (Groups): what the disclosure counts.
// - actions(x): undo {plan, label, text, tip} and again {plan, label}. tip is undefined unless the row has its own
//   wording (the buttons then use their default).
// - empty(x): what Undo says when there is nothing to undo.
const TAKE = {
  route: (e) => ({ lead: null, src: srcOf(e), dst: dstItem(e.dst, e.dstName, e.router), many: null, notes: [] }),
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
    return { undo, again: { plan: again, label: `${againWord(again)} ${src.label} to ${dst.label}` } };
  },
  empty: (e) => `Nothing was routed before #${e.id}.`,
};

// A Group of takes: a take that sent several routes.
const GROUP = {
  route(f) {
    // The hop entries of a tie take (tie) are part of the routes asked for, so the row names only those routes.
    const asked = f.entries.filter((e) => !e.tie);
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
    ];
    return {
      lead: null,
      src: srcs.size === 1 ? srcOf(es[0]) : `${srcs.size} sources`,
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
