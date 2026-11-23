// Pure rules for the Settings form (Node-tested).

// entries come from the form's own inputs, not the built config. A blank Auto-lock time means 0 (never).
export function intProblem(entries) {
  for (const e of entries) {
    if (e.type !== 'int') continue;
    const v = String(e.value ?? '').trim();
    if (v === '' && e.key === 'autoLockMinutes') continue;
    if (v === '' || !Number.isInteger(Number(v))) return e;
  }
  return null;
}

// Go sends a zero time ("0001-…") for "never".
const realTime = (iso) => !!iso && new Date(iso).getFullYear() > 2000;

// The Router names time: a finished read wins over the saved names table (savedAt).
export function namesTimeText({ at, savedAt }, fmt) {
  if (realTime(at)) return `Re-read ${fmt(at)}`;
  if (realTime(savedAt)) return `Names from ${fmt(savedAt)}`;
  return 'Not read yet';
}
