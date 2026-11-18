// Pure parts of the Export tab (exportdialog.js). The list comes from GET api/export/items; the bodies go to
// POST api/export. Category names and item ids are the server's (internal/app/exportfile.go, exportitems.go).

// In file order; keep in step with exportCategories in exportfile.go (exportrules.test.mjs reads it).
export const CATEGORIES = [
  ['settings', 'Settings'],
  ['routers', 'Routers'],
  ['users', 'Users'],
  ['presets', 'Presets'],
  ['filterSets', 'Filter sets'],
  ['panels', 'Panels'],
  ['artwork', 'Artwork'],
  ['apiClients', 'API clients'],
  ['browser', 'This browser'],
];

// Picked item by item.
export const ITEM_CATS = ['users', 'presets', 'filterSets', 'panels', 'artwork', 'apiClients'];

export const KIND_LABEL = { panel: 'Panel', template: 'Template', checkpointSet: 'Checkpoint set', default: 'Default artwork', icon: 'Icon' };

const catOf = (list, name) => list.categories.find((c) => c.name === name) || { name, size: 0, items: [], sections: [] };

// Everything ticked, Include PINs on.
export function newPick(list) {
  const items = {};
  for (const c of ITEM_CATS) items[c] = new Set((catOf(list, c).items || []).map((i) => i.id));
  return { sections: new Set((catOf(list, 'settings').sections || []).map((s) => s.id)), items, routers: true, browser: true, pins: true };
}

// 'all', 'some' or 'none' for the category's box.
export function catState(list, pick, cat) {
  let total, on;
  if (cat === 'settings') [total, on] = [(catOf(list, cat).sections || []).length, pick.sections.size];
  else if (cat === 'routers') return pick.routers ? 'all' : 'none';
  else if (cat === 'browser') return pick.browser ? 'all' : 'none';
  else [total, on] = [(catOf(list, cat).items || []).length, pick.items[cat].size];
  if (!on) return 'none';
  return on === total ? 'all' : 'some';
}

export function setCat(list, pick, cat, on) {
  if (cat === 'settings') pick.sections = new Set(on ? catOf(list, cat).sections.map((s) => s.id) : []);
  else if (cat === 'routers') pick.routers = on;
  else if (cat === 'browser') pick.browser = on;
  else pick.items[cat] = new Set(on ? (catOf(list, cat).items || []).map((i) => i.id) : []);
}

export function toggle(set, id, on) {
  if (on) set.add(id);
  else set.delete(id);
}

// The first picked item that names the router, or null. The server sends the router with it whether Routers is
// ticked or not.
export function routerBroughtBy(list, pick) {
  for (const c of ITEM_CATS) for (const it of catOf(list, c).items || []) if (it.router && pick.items[c].has(it.id)) return it;
  return null;
}

// Uncompressed bytes of what is ticked in cat. browserBytes: This browser's values, measured by the page.
export function catSize(list, pick, cat, browserBytes = 0) {
  const c = catOf(list, cat);
  if (cat === 'settings') return (c.sections || []).filter((s) => pick.sections.has(s.id)).reduce((n, s) => n + s.size, 0);
  if (cat === 'routers') return pick.routers || routerBroughtBy(list, pick) ? c.size : 0;
  if (cat === 'browser') return pick.browser ? browserBytes : 0;
  return (c.items || []).filter((i) => pick.items[cat].has(i.id)).reduce((n, i) => n + i.size, 0);
}

export const nothingPicked = (list, pick) => CATEGORIES.every(([c]) => catState(list, pick, c) === 'none');

// The POST api/export body for the ticks. Every item category sends its ids, so an item added meanwhile does not
// go unseen.
export function exportBody(list, pick, browser) {
  const body = { categories: [], pick: {}, sections: [], includePins: pick.pins };
  for (const [c] of CATEGORIES) {
    if (catState(list, pick, c) === 'none') continue;
    body.categories.push(c);
    if (c === 'settings') body.sections = [...pick.sections];
    if (ITEM_CATS.includes(c)) body.pick[c] = [...pick.items[c]];
    if (c === 'browser') body.browser = browser;
  }
  return body;
}

// Back up everything: every category whole, with PINs, whatever the ticks say.
export function backupBody(browser) {
  return { categories: CATEGORIES.map(([c]) => c), includePins: true, browser };
}

export function sizeText(bytes) {
  if (bytes < 1000) return `${bytes} B`;
  if (bytes < 1000_000) return `${(bytes / 1000).toFixed(bytes < 10_000 ? 1 : 0)} kB`;
  return `${(bytes / 1000_000).toFixed(1)} MB`;
}

// The file name from Content-Disposition, else a dated default.
export function fileNameOf(disposition, today) {
  const m = /filename="([^"]+)"/.exec(disposition || '');
  return m ? m[1] : `${today}-Sode-Export.zip`;
}
