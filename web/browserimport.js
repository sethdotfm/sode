// The values are another browser's, so only known values pass.

import { PREFS } from './prefs.js';

export function browserPrefs(file) {
  const out = {};
  for (const [name, p] of Object.entries(PREFS)) {
    const v = file?.[name];
    if (typeof v === 'string' && Object.hasOwn(p.values, v)) out[name] = v;
  }
  return out;
}

// Names match without regard to case.
export function mergeByName(local, file, replace) {
  if (replace) return file;
  const key = (x) => String(x.name).trim().toLowerCase();
  const out = local.map((x) => file.find((f) => key(f) === key(x)) || x);
  return out.concat(file.filter((f) => !local.some((x) => key(x) === key(f))));
}
