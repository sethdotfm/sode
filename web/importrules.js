// Pure parts of the Import tab (importtab.js). The check and the plan come from POST api/import; the request goes
// back to it for the review and to POST api/import/apply. Item keys and choice values are the server's
// (importItem and the clash choices in internal/app/importplan.go).

// The categories the server plans, in its order. This browser the page applies itself (browserimport.js). Profile is
// only in an own import (internal/app/ownexport.go).
export const PLANNED = ['profile', 'settings', 'routers', 'users', 'presets', 'filterSets', 'apiClients', 'artwork', 'panels', 'browser'];

const CHOICES = [
  ['new', 'Use new'],
  ['current', 'Use current'],
  ['skip', 'Skip'],
  ['rename', 'Rename'],
];
// The server turns Rename into Use new for these (importPlanner.choice), so the page does not offer it. Default
// artwork is one per item, so it has no Rename either.
const NO_RENAME = new Set(['settings', 'routers']);

export const choicesFor = (cat, it) => (NO_RENAME.has(cat) || it?.kind === 'default' ? CHOICES.slice(0, 3) : CHOICES);

const catOf = (check, cat) => check.plan.categories[cat] || { items: [] };

// A clash the person decides: live, different, and in a category that imports. Replace import takes the file's
// item always, so it has none.
export const hasChoice = (c, it, pick) => pick?.mode !== 'replace' && !c.refused && !it.refused && !it.same && !it.remove && !!it.clash && !!it.choice;

// Everything ticked but Routers, which changes the router only when ticked, with the preselected choices. Add
// import, with Keep current Administrators ticked for when the person picks Replace import. empty holds the ticked
// categories the file holds with no items: only Replace import sends them, to empty them here.
export function newPick(check) {
  const pick = { mode: 'add', keepAdmins: true, items: {}, choices: {}, empty: new Set() };
  for (const cat of PLANNED) {
    const c = catOf(check, cat);
    if (!c.items.length && !c.refused && cat !== 'routers' && check.categories?.includes(cat)) pick.empty.add(cat);
    pick.items[cat] = new Set(cat === 'routers' || c.refused ? [] : c.items.map((i) => i.key));
    pick.choices[cat] = {};
    for (const it of c.items) if (hasChoice(c, it)) pick.choices[cat][it.key] = it.choice;
  }
  return pick;
}

// 'all', 'some' or 'none' for the category's box.
export function catState(check, pick, cat) {
  const total = catOf(check, cat).items.length;
  if (!total) return pick.empty?.has(cat) ? 'all' : 'none';
  const on = pick.items[cat]?.size || 0;
  if (!on) return 'none';
  return on === total ? 'all' : 'some';
}

export function setCat(check, pick, cat, on) {
  const c = catOf(check, cat);
  if (!c.items.length && !c.refused) {
    if (on) pick.empty.add(cat);
    else pick.empty.delete(cat);
  }
  pick.items[cat] = new Set(on && !c.refused ? c.items.map((i) => i.key) : []);
}

export const catSize = (check, pick, cat) =>
  catOf(check, cat)
    .items.filter((i) => pick.items[cat]?.has(i.key))
    .reduce((n, i) => n + (i.size || 0), 0);

// The importReq. A category with nothing ticked is left out, since an empty pick means every item to the server.
export function importReq(pick) {
  const replace = pick.mode === 'replace';
  const req = { mode: replace ? 'replace' : 'add', categories: [], sections: [], pick: {}, choices: {} };
  if (replace && pick.keepAdmins === false) req.keepAdmins = false;
  for (const cat of PLANNED) {
    const keys = [...(pick.items[cat] || [])];
    if (replace && !keys.length && pick.empty?.has(cat)) {
      req.categories.push(cat); // no pick: the file's none
      continue;
    }
    if (!keys.length) continue;
    req.categories.push(cat);
    if (cat === 'settings') req.sections = keys;
    else if (cat !== 'routers') req.pick[cat] = keys;
    const ch = replace ? [] : Object.entries(pick.choices[cat] || {}).filter(([k]) => pick.items[cat].has(k));
    if (ch.length) req.choices[cat] = Object.fromEntries(ch);
  }
  return req;
}

// The names Use new writes over (importItem.overwrites). Any makes the import Critical.
export function criticalNames(check, pick) {
  const out = [];
  for (const cat of PLANNED) {
    const c = catOf(check, cat);
    for (const it of c.items) if (pick.items[cat]?.has(it.key) && hasChoice(c, it) && pick.choices[cat][it.key] === 'new') out.push(it.name);
  }
  return out;
}

// importItem.writes, for a planned item.
const writes = (it) => !it.same && !it.refused && !it.remove && (!it.clash || it.choice === 'new' || it.choice === 'rename');

const plural = (n, w) => `${n} ${w}`;

// One line per planned category: what the import does to it.
export function reviewCounts(plan) {
  const out = [];
  for (const cat of PLANNED) {
    const c = plan.categories[cat];
    if (!c) continue;
    if (c.refused) {
      out.push({ cat, text: `Not imported: ${c.refused}` });
      continue;
    }
    const n = { added: 0, updated: 0, removed: 0, kept: 0, 'no change': 0, 'not imported': 0, Broken: 0 };
    for (const it of c.items) {
      if (it.remove) n.removed++;
      else if (it.same) n['no change']++;
      else if (it.refused) n['not imported']++;
      else if (!writes(it)) n.kept++;
      else if (it.clash && it.choice === 'new') n.updated++;
      else n.added++;
      if (writes(it) && it.broken?.length) n.Broken++;
    }
    const text = Object.entries(n)
      .filter(([, v]) => v)
      .map(([k, v]) => plural(v, k))
      .join(', ');
    out.push({ cat, text: text || 'Nothing' });
  }
  return out;
}

// The items that arrive Broken, with what each is missing.
export function brokenItems(plan) {
  const out = [];
  for (const cat of PLANNED) {
    const c = plan.categories[cat];
    if (c && !c.refused) for (const it of c.items) if (writes(it) && it.broken?.length) out.push({ cat, ...it });
  }
  return out;
}

// offset: the shown number of source or destination 0.
export function missingText(m, offset) {
  const r = m.ref || {};
  switch (m.kind) {
    case 'source':
    case 'destination':
      return r.name ? `${m.kind} ${r.name} (${r.n + offset})` : `${m.kind} ${r.n + offset}`;
    case 'level':
      return `level ${r.n}`;
    case 'router':
      return `router ${r.router}`;
  }
  return `${m.kind} ${r.name || ''}`.trim();
}
