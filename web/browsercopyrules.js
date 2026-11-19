// Pure (docs/agents/profiles.md).

import { PREFS } from './prefs.js';

// The prefs a profile holds: config.PrefValues, which TestPrefTable keeps in step with PREFS.
const ME_PREFS = Object.keys(PREFS);

// values: browserValues() from exportcalls.js. A pref with no Me counterpart is shown, not copied.
export function copyDoc(values, mePrefs = ME_PREFS) {
  const prefs = {};
  const browserOnly = [];
  for (const [name, value] of Object.entries(values)) {
    if (name === 'filterSets' || name === 'presets') continue;
    if (mePrefs.includes(name)) prefs[name] = value;
    else browserOnly.push({ name, value });
  }
  return { prefs, browserOnly, filterSets: values.filterSets || [], presets: values.presets || [] };
}

const MOVED = new Set(['import', 'replace', 'both']);

// Skip and Use current did not reach Me, so this browser keeps them. filterSets come back as moves so an active set follows its new name.
export function toClear({ prefs, rows }, keep) {
  const out = { prefs: [], filterSets: [], presets: [] };
  if (keep) return out;
  for (const p of prefs) if (p.on) out.prefs.push(p.name);
  for (const r of rows) {
    if (!MOVED.has(r.choice)) continue;
    if (r.kind === 'filter') out.filterSets.push({ from: r.name, to: r.as || r.name });
    else out.presets.push(r.name);
  }
  return out;
}

// A moved preset keeps its id (reviewsheet.js), so its pin key changes scope only.
export const movedPinOrder = (order, ids) => order.map((k) => (k.startsWith('local:') && ids.includes(k.slice(6)) ? `user:${k.slice(6)}` : k));

export const moveText = (what, keep) => (keep ? `${what} copied to Me. This browser keeps them too.` : `${what} moved to Me.`);
