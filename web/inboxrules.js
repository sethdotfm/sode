// Card views, the plan and request bodies are the server's (internal/app/inboxsend.go).

import { itemRefs } from './presetref.js';

export const CARD_CATS = ['presets', 'filterSets', 'panels'];
const WORDS = { presets: ['preset', 'presets'], filterSets: ['filter set', 'filter sets'], panels: ['panel', 'panels'] };

const STATES = { waiting: 'Waiting', declined: 'Declined', expired: 'Expired', withdrawn: 'Withdrawn' };

export function stateText(c) {
  if (c.state !== 'accepted') return STATES[c.state] || c.state;
  const n = c.accepted || 0;
  return n === c.count ? 'Accepted' : `Accepted ${n} of ${c.count}`;
}

export const fromText = (c) => `From ${c.from || 'a removed user'}`;

export const countLabel = (n) => (n ? `Inbox, ${n} ${n === 1 ? 'card' : 'cards'} waiting` : 'Inbox');

export function itemsText(items) {
  const parts = [];
  for (const cat of CARD_CATS) {
    const n = items.filter((it) => it.category === cat).length;
    if (n) parts.push(`${n} ${WORDS[cat][n === 1 ? 0 : 1]}`);
  }
  return parts.join(', ');
}

// One toast per new card. A card view carries its items while it waits.
export function toastText(c) {
  const what = c.items?.length ? itemsText(c.items) : `${c.count} ${c.count === 1 ? 'item' : 'items'}`;
  return `${c.from || 'A removed user'} sent you ${what}.`;
}

export function acceptPick(plan) {
  const pick = { items: {}, choices: {} };
  for (const cat of CARD_CATS) {
    const c = plan.categories[cat];
    pick.items[cat] = new Set(c && !c.refused ? c.items.map((it) => it.key) : []);
    pick.choices[cat] = {};
    if (c && !c.refused) for (const it of c.items) if (it.clash && it.choice && !it.same) pick.choices[cat][it.key] = it.choice;
  }
  return pick;
}

// The plan part of inboxReview and inboxAccept. Every category goes, empty or not: the server reads a missing one as every item.
export function acceptReq(pick) {
  const req = { pick: {}, choices: {} };
  for (const cat of CARD_CATS) {
    const keys = [...(pick.items[cat] || [])];
    req.pick[cat] = keys;
    const ch = Object.entries(pick.choices[cat] || {}).filter(([k]) => pick.items[cat].has(k));
    if (ch.length) req.choices[cat] = Object.fromEntries(ch);
  }
  return req;
}

export const pickedCount = (pick) => CARD_CATS.reduce((n, cat) => n + (pick.items[cat]?.size || 0), 0);

// This browser items go whole, presets in the stored form (presetref.js); sets are filterstore.js entries.
export function sendBody(router, to, { presets = [], filterSets = [], panels = [] }) {
  const stored = ({ id, name, items, pinned }) => ({ id, name, pinned, items: items.map((it) => itemRefs(it, router)) });
  return {
    to,
    presets: presets.map((p) => (p.scope === 'local' ? { scope: p.scope, preset: stored(p) } : { scope: p.scope, id: p.id })),
    filterSets: filterSets.map((s) => (s.scope === 'local' ? { scope: s.scope, set: { name: s.name, ...s.f } } : { scope: s.scope, id: s.id })),
    panels: panels.map((p) => p.id),
  };
}
