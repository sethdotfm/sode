// Config the server owns: lists with their own save messages and size caps
// (filter sets, salvos, users and locks). The Settings form must not
// send them back with the rest: they can be tens of KiB each, the server's
// read limit is 64 KiB, and the server ignores them in a `config` message.
// Pure, so Node tests load it.

export const OWNED_KEYS = ['filterSets', 'salvos', 'users', 'locks', 'autoLockMinutes'];

// cfg as a plain copy without the owned keys.
export function omitOwned(cfg) {
  const out = { ...cfg };
  for (const k of OWNED_KEYS) delete out[k];
  return out;
}
