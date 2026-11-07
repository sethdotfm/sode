// inForce: value in effect; mine: the user's profile value ('' = none). Save shows only if it would change the profile; Clear only if it holds something.
export function myDefaultButtons(inForce, mine) {
  const m = mine || '';
  return { save: inForce !== m, clear: m !== '' };
}
