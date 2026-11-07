// Pure rules for the Settings form (Node-tested).

// `entries` are read from the form's own inputs, not the built config (readSettings leaves out unchanged keys). A blank idle time means 0 (never).
export function intProblem(entries) {
  for (const e of entries) {
    if (e.type !== 'int') continue;
    const v = String(e.value ?? '').trim();
    if (v === '' && e.key === 'autoLockMinutes') continue;
    if (v === '' || !Number.isInteger(Number(v))) return e;
  }
  return null;
}
