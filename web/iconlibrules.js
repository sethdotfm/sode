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
