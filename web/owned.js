// Owned config: lists with their own save messages and caps (filter sets, presets, users). The Settings form must not send them back:
// tens of KiB each against the 64 KiB read limit, and the server ignores them in `config`. Pure.

export const OWNED_KEYS = ['filterSets', 'presets', 'users', 'maxPinLen', 'signInMode'];

export function omitOwned(cfg) {
  const out = { ...cfg };
  for (const k of OWNED_KEYS) delete out[k];
  return out;
}
