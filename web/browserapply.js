// Writes a file's values to this browser; pure parts are in browserimport.js.

import { browserPrefs, mergeByName } from './browserimport.js';
import { setPref } from './device.js';
import { localSets, normFilter, putLocalSets } from './filterstore.js';
import { localPresets, putLocalPresets, storedPreset } from './presetstore.js';

export function applyBrowser(values, replace) {
  for (const [name, v] of Object.entries(browserPrefs(values))) setPref(name, v);
  const list = (x) => [].concat(x || []).filter((s) => s?.name);
  const sets = list(values?.filterSets).map((s) => ({ scope: 'local', name: String(s.name), f: normFilter(s.f) }));
  putLocalSets(mergeByName(localSets(), sets, replace));
  const presets = list(values?.presets).map((s) => ({ scope: 'local', ...storedPreset(s), pinned: s.pinned !== false }));
  putLocalPresets(mergeByName(localPresets(), presets, replace));
}
