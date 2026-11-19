// The review's backup offer (spec #543, The one Save). It is a suggestion, never a step of the save. Pure (Node-tested).

import { ago } from './keyrules.js';

// The `exportSession` gate is Administrator (internal/app/policy.go), so another Role's download would be refused.
// interruptions null: Sode did not answer, so what the save stops is unknown.
export function offersBackup({ saves, isCritical, interruptions, role }) {
  if (role !== 'administrator' || !saves.length) return false;
  return saves.some(isCritical) || !interruptions || interruptions.length > 0;
}

// iso: when this browser last downloaded a full backup, or null.
export function lastBackupText(iso, now) {
  const t = Date.parse(iso || '');
  return `Last backup: ${Number.isNaN(t) ? 'never' : ago(t, now)} in this browser`;
}
