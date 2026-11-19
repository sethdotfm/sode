// The server works out the list (internal/app/interruptions.go). Pure, no imports (Node-tested).

const COUNTS = [
  ['panels', 'panel', 'panels'],
  ['streams', 'stream', 'streams'],
  ['sessions', 'session', 'sessions'],
  ['keys', 'key', 'keys'],
  ['stepIns', 'step-in', 'step-ins'],
];

const sum = (list, k) => list.reduce((n, x) => n + (x[k] || 0), 0);
const words = (n, one, many) => `${n} ${n === 1 ? one : many}`;

// What the save stops and puts on a lock screen, or '' when nothing: "stop 3 panels, 1 session and lock 2 browsers".
export function stopsText(list) {
  const stop = COUNTS.map(([k, one, many]) => [sum(list, k), one, many])
    .filter(([n]) => n > 0)
    .map(([n, one, many]) => words(n, one, many));
  const browsers = sum(list, 'browsers');
  const parts = [];
  if (stop.length) parts.push(`stop ${stop.join(', ')}`);
  if (browsers) parts.push(`lock ${words(browsers, 'browser', 'browsers')}`);
  return parts.join(' and ');
}

// The Save button. save and keep: the counts of changes that save and that stay unsaved (savereviewrules.js saveLabel
// when nothing stops).
export function saveStopsLabel(save, keep, list, plain) {
  const stops = stopsText(list || []);
  if (!save || !stops) return plain;
  if (keep) return `Save ${save} and ${stops}, keep ${keep} unsaved`;
  return `Save and ${stops}`;
}

export const endsSomething = (list) => (list || []).some((x) => x.ends);

export const NO_RETURN = 'Keys, streams and sessions that this save ends do not come back, even after a restore.';
