// Pure parts of importing This browser (importtab.js applies them). The file's values come from POST api/import's
// `browser`, in the form exportcalls.js browserValues writes. They are another browser's, so only known values pass.

import { PREFS } from './prefs.js';

// The file's preferences that this Sode knows, with values it knows.
export function browserPrefs(file) {
  const out = {};
  for (const [name, p] of Object.entries(PREFS)) {
    const v = file?.[name];
    if (typeof v === 'string' && Object.hasOwn(p.values, v)) out[name] = v;
  }
  return out;
}

// Add import puts each file entry over the local one of its name (ignoring case) and keeps the others. Replace
// import keeps only the file's.
export function mergeByName(local, file, replace) {
  if (replace) return file;
  const key = (x) => String(x.name).trim().toLowerCase();
  const out = local.map((x) => file.find((f) => key(f) === key(x)) || x);
  return out.concat(file.filter((f) => !local.some((x) => key(x) === key(f))));
}
