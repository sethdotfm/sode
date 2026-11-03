// The lock screen's pure rules (#151): PIN entry, when to ask the server,
// idle time and where the logo flies from. No imports, so Node tests them.

// Typed after the last key before a PIN that could still grow is sent.
export const MORE_DIGITS_WAIT = 500;

// The PIN after a pad or keyboard key: a digit, 'back' or 'clear'. lens are
// the users' PIN lengths; nothing longer than the longest can be right.
export function padKey(pin, key, lens) {
  if (key === 'clear') return '';
  if (key === 'back') return pin.slice(0, -1);
  if (/^[0-9]$/.test(key) && pin.length < Math.max(...lens)) return pin + key;
  return pin;
}

// How long to wait before sending pin to the server, or null for not at all.
// lens are the users' PIN lengths. Only a length some user has can match, so
// nothing else is sent. If a longer PIN exists this one may be the start of
// it: wait a moment in case more digits come, so a slow typist's first four
// digits aren't counted as a wrong try. The longest is sent at once.
export function sendDelay(pin, lens) {
  const n = pin.length;
  if (!lens.includes(n)) return null;
  return Math.max(...lens) > n ? MORE_DIGITS_WAIT : 0;
}

// Whether the idle time has run out; 0 minutes = never.
export const idleDue = (lastActive, now, minutes) => minutes > 0 && now - lastActive >= minutes * 60_000;

// Which kind of page this is, by its path.
export function pageType(pathname) {
  if (/\/log(\.html)?\/?$/.test(pathname)) return 'log';
  if (/\/grid(\.html)?\/?$/.test(pathname)) return 'grid';
  return 'main';
}

// The transform that puts the box `to` (where the logo ends up) over the box
// `from` (where the page's own logo is): translate its centre, scale its width.
export function liftFrom(from, to) {
  if (!from || !from.width || !to || !to.width) return { x: 0, y: 0, scale: 1 };
  return {
    x: from.left + from.width / 2 - (to.left + to.width / 2),
    y: from.top + from.height / 2 - (to.top + to.height / 2),
    scale: from.width / to.width,
  };
}

// Dots for the PIN typed so far.
export const dots = (pin) => '●'.repeat(pin.length);
