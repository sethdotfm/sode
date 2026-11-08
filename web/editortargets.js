// What the Filters and Presets editors work on: This browser's own data, or a draft aimed at one
// user's profile (t is profile.js editTarget). `edit` and `saver` are null for the browser:
// profileSaver and scopeWriter take them as "no profile".

import { nameProblem } from './core.js';
import { upsertEntry, filterEntry } from './profilesrules.js';

const forUser = (noun, t) => `${t.old ? 'Edit' : 'Add'} ${noun} for ${t.user}`;

// Filters. read/write: the browser's filter. snapshot/restore: Cancel rolls the filter back, since
// edits reach History as they are made. onClose: the browser's saver settles.
export function browserFilters({ read, write, snapshot, restore, onClose }) {
  let snap = null;
  return {
    own: true,
    edit: null,
    saver: null,
    title: 'Filters',
    name: '',
    filter: read,
    put: write,
    begin() {
      snap = snapshot();
    },
    landed() {
      snap = snapshot();
    },
    // flush settles the editors' pending text before the restore would be overwritten by it.
    cancel(flush, close) {
      if (!snap) return close(true);
      flush();
      restore(snap);
      close(false);
    },
    close: onClose,
    build: () => ({ err: 'Not editing a profile.' }),
  };
}

export function profileFilters(t, { normalize, saver }) {
  let f = normalize(t.found);
  return {
    own: false,
    edit: t,
    saver,
    title: forUser('filter set', t),
    name: t.old,
    filter: () => f,
    put(next) {
      f = normalize(next);
    },
    begin() {},
    landed() {},
    cancel: (flush, close) => close(true),
    close() {},
    // The whole next list for the profile, or the reason the name will not do.
    build(name) {
      const err = nameProblem(
        name,
        t.lists.map((x) => String(x?.name)),
        { noun: 'filter set', old: t.old, reserved: 'all' },
      );
      return err ? { err } : { list: upsertEntry(t.lists, filterEntry(name, f), t.old) };
    },
  };
}

// Presets. list(scope): the scope's presets. note(cur): the line under the name. save(built,
// moving): writes the built preset where this target keeps them.
export function browserPresets({ list, note, save, onClose }) {
  return {
    own: true,
    edit: null,
    saver: null,
    title: 'Edit Presets',
    list,
    note,
    save,
    close: onClose,
  };
}

export function profilePresets(t, { save, saver }) {
  return {
    own: false,
    edit: t,
    saver,
    title: forUser('preset', t),
    list: () => t.lists,
    note: () => '',
    save,
    close() {},
  };
}
