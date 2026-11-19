// Parts of Settings outside the form that save with the one Save: Manage users and the API section
// (docs/agents/settings-saves.md). settingssave.js reads them; each module registers its own, so neither imports
// the other.
//
// A source has these functions. owns(key). base() and now(): the fields as loaded and as edited ({key: value}).
// version(key). label(row) and show(row, value): for the review. problem(): a text that stops the review, or ''.
// saved(key, value, result): after a field saved or was unchanged. conflict(key, result). reset(): drops its edits.

const list = [];

export const addSaveSource = (s) => list.push(s);
export const saveSources = () => list;
export const sourceOf = (key) => list.find((s) => s.owns(key));
