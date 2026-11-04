// Which "my default" buttons a This device row shows (pure, Node-tested).
// `inForce` is the value now in effect for the pref, `mine` the value the
// logged-in user's profile holds ('' = none). Save shows only when it would
// change the profile; Clear only when the profile holds something.
export function myDefaultButtons(inForce, mine) {
  const m = mine || '';
  return { save: inForce !== m, clear: m !== '' };
}
