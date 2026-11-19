// A suggestion, never a step of the save (docs/agents/settings-saves.md, The one Save). Pure (Node-tested).

import { ago } from './keyrules.js';

// exportSession needs an Administrator (policy.go), so the server refuses another Role's download. interruptions null: Sode did not answer.
export function offersBackup({ saves, isCritical, interruptions, role }) {
  if (role !== 'administrator' || !saves.length) return false;
  return saves.some(isCritical) || !interruptions || interruptions.length > 0;
}

// iso: the last full backup from this browser, or null.
export function lastBackupText(iso, now) {
  const t = Date.parse(iso || '');
  return `Last backup: ${Number.isNaN(t) ? 'never' : ago(t, now)} in this browser`;
}
