// The Icon library's pure rules: names and the Choose icon picker's sections. Pure. The server checks names again
// (freeName in internal/app/iconlib.go), and a clash there becomes "Name 2".
import { artFilter } from './sidebarrules.js';

export const LIB_NAME_MAX = 40; // characters, as iconNameMax in internal/app/iconlib.go
export const LIB_TYPES = ['image/png', 'image/svg+xml'];

export function libNameProblem(name) {
  const n = String(name ?? '').trim();
  if (!n) return 'Give the icon a name.';
  if ([...n].length > LIB_NAME_MAX) return `An icon name is at most ${LIB_NAME_MAX} characters.`;
  return '';
}

// The name an uploaded file gets unless one is typed.
export const libNameOf = (fileName) =>
  [
    ...String(fileName)
      .replace(/\.[^.]*$/, '')
      .trim(),
  ]
    .slice(0, LIB_NAME_MAX)
    .join('')
    .trim();

// uses is {cells, panels} from GET /icon-library/{id}/uses, or null when it could not be read. Cells keep `lib:<id>`
// after a delete and draw no Icon A (apiIconDelete).
export function libDeleteText(name, uses) {
  const ask = `Delete ${name} from the Icon library?`;
  if (!uses) return `${ask} Cells that use it will draw no Icon A; their Icon B and text stay.`;
  const { cells, panels } = uses;
  if (!cells) return `${ask} No cell uses it.`;
  const one = cells === 1;
  return `${ask} ${cells} ${one ? 'cell' : 'cells'} on ${panels} ${panels === 1 ? 'panel' : 'panels'} ${one ? 'uses' : 'use'} it and will draw no Icon A; ${one ? 'its' : 'their'} Icon B and text stay.`;
}

// res is {added, skipped} from POST /icon-library/rescan; a skipped file stays in icons/drop-in until it is fixed.
export function libRescanText(res) {
  const added = res?.added || [];
  const skipped = res?.skipped || [];
  const n = (k, one, many) => `${k} ${k === 1 ? one : many}`;
  const parts = [];
  if (added.length) parts.push(`Added ${n(added.length, 'icon', 'icons')}: ${added.map((ic) => ic.name).join(', ')}.`);
  if (skipped.length) parts.push(`Skipped ${n(skipped.length, 'file', 'files')}: ${skipped.map((s) => `${s.file} (${s.reason})`).join(', ')}.`);
  return parts.join(' ') || 'Rescan found no files in icons/drop-in.';
}

// Uploaded first (library icons by name, ignoring case), then the vendored icons, both cut to the search. Uploaded is
// always there, empty or not, since it holds the Upload button.
export function pickerSections(library, vendored, text) {
  const lib = (Array.isArray(library) ? library : [])
    .filter((ic) => artFilter([ic.name.toLowerCase()], text).length)
    .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }))
    .map((ic) => ({ value: `lib:${ic.id}`, name: ic.name, id: ic.id }));
  return [
    { id: 'uploaded', tiles: lib },
    { id: 'icons', tiles: artFilter(vendored, text).map((n) => ({ value: `icon:${n}`, name: n })) },
  ];
}
