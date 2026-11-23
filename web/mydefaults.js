// inForce: the value in effect. mine: the user's Profile value ('' = none).
export function myDefaultButtons(inForce, mine) {
  const m = mine || '';
  return { save: inForce !== m, clear: m !== '' };
}
