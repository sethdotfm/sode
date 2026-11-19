// Words for the Request approval cards (approvalcards.js). Pure, no imports.

// A change from the server's SavedField (internal/app/savelog.go): secrets show only as changed.
export function changeText(f) {
  const v = (x) => (typeof x === 'string' ? x : JSON.stringify(x));
  const parts = [];
  if (f.changed) parts.push('changed');
  if (f.old !== undefined || f.new !== undefined) parts.push(`${f.old === undefined ? 'nothing' : v(f.old)} to ${f.new === undefined ? 'nothing' : v(f.new)}`);
  if (f.added?.length) parts.push(`added ${f.added.map(v).join(', ')}`);
  if (f.removed?.length) parts.push(`removed ${f.removed.map(v).join(', ')}`);
  return `${f.label}: ${parts.join('; ')}`;
}

// ms left as m:ss
export function leftText(ms) {
  const s = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// How a request ended, as the cards say it. '' when a card goes with no text (its browser stopped qualifying).
export function endText(end, by) {
  switch (end) {
    case 'approved':
      return `Approved by ${by}`;
    case 'denied':
      return `Denied by ${by}`;
    case 'timeout':
      return 'No one answered';
    case 'cancelled':
      return 'Cancelled';
  }
  return '';
}

// The Role names the server sends in `approvers` for the Role an action needs.
export const approverKey = (who) => (who === 'an Administrator' ? 'administrator' : 'trusted');
