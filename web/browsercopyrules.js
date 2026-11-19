// Copy this browser to My user (docs/agents/profiles.md). Pure: what goes to Me, and what the
// move then takes off this browser.

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

// prefs: [{name, on}], on false when the user picked Skip. rows: [{kind, name, choice}] with the
// review's choices. Skip and Use current did not reach Me, so this browser keeps them.
export function toClear({ prefs, rows }, keep) {
  const out = { prefs: [], filterSets: [], presets: [] };
  if (keep) return out;
  for (const p of prefs) if (p.on) out.prefs.push(p.name);
  for (const r of rows) if (MOVED.has(r.choice)) out[r.kind === 'filter' ? 'filterSets' : 'presets'].push(r.name);
  return out;
}

export const moveText = (what, keep) => (keep ? `${what} copied to Me. This browser keeps them too.` : `${what} moved to Me.`);
