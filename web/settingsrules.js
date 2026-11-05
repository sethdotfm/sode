// Pure rules for the Settings form (Node-tested).

// The first whole-number field whose typed value isn't one, or null.
// `entries` are { key, type, value } read from the form's own inputs, not from
// the config built from them: readSettings leaves out unchanged keys. A blank
// idle time means 0 (never); any other blank number is wrong.
export function intProblem(entries) {
  for (const e of entries) {
    if (e.type !== 'int') continue;
    const v = String(e.value ?? '').trim();
    if (v === '' && e.key === 'autoLockMinutes') continue;
    if (v === '' || !Number.isInteger(Number(v))) return e;
  }
  return null;
}
