// Pure parts of Send a copy and the Inbox (inbox.js, inboxdialog.js, sendcopy.js). Card views, the plan and the
// request bodies are the server's (cardView, importPlan and inboxReq in internal/app/inboxsend.go).

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

// Every item ticked, with the server's choice for each clash. A refused category ticks nothing.
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

// The plan part of inboxReview and inboxAccept. Every category goes, empty or not: the server reads a missing one as
// every item (cardPlanner.picked).
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

// The inboxSend body. Me and Everyone items go by id (presets) or name (filter sets); This browser items go whole.
// sets are filterstore.js entries ({ scope, name, f }).
export function sendBody(to, { presets = [], filterSets = [], panels = [] }) {
  return {
    to,
    presets: presets.map(({ scope, ...p }) => (scope === 'local' ? { scope, preset: p } : { scope, id: p.id })),
    filterSets: filterSets.map((s) => (s.scope === 'local' ? { scope: s.scope, set: { name: s.name, ...s.f } } : { scope: s.scope, name: s.name })),
    panels: panels.map((p) => p.id),
  };
}
