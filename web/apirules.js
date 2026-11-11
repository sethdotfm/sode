// API settings and scopes, pure (no imports) so Node tests them. Keep in step with the validation of
// config.APISettings and with config.AllScopes in internal/config/api.go.

export const SCOPES = [
  { id: 'names', text: 'Names' },
  { id: 'tally', text: 'Tally' },
  { id: 'protected', text: 'Protected destinations' },
  { id: 'status', text: 'Status' },
  { id: 'presets', text: 'Presets' },
  { id: 'history', text: 'History' },
  { id: 'layouts', text: 'Panel layouts' },
  { id: 'log', text: 'Communication Log' },
  { id: 'take', text: 'Takes, Undo, Redo and Retry' },
  { id: 'panels', text: 'Read panels' },
  { id: 'panelsmanage', text: 'Manage panels' },
];

export function apiProblem(a) {
  if (!Number.isInteger(a.rateLimit) || a.rateLimit < 0 || a.rateLimit > 100000) return 'The rate limit is 0 (none) to 100000 requests a minute.';
  if (!Number.isInteger(a.takeCap) || a.takeCap < 1 || a.takeCap > 64) return 'The take cap is 1 to 64.';
  return '';
}

const FIELDS = ['enabled', 'rateLimit', 'takeCap', 'guestEnabled', 'logReads'];

// A field left out keeps the server's value.
export function apiSettingsReq(cur, next) {
  const apiSettings = {};
  for (const k of FIELDS) if (next[k] !== cur[k]) apiSettings[k] = next[k];
  return { t: 'apiSettings', apiSettings };
}

// Whether the form differs from the saved settings.
export const apiEdited = (cur, next) => FIELDS.some((k) => next[k] !== cur[k]);

// A hello repaints unless the form holds edits that are neither the saved values nor the incoming ones.
// The commit's hello can land before the save result, so a form equal to the incoming values is not an edit.
export const apiRepaints = (saved, form, incoming) => !apiEdited(saved, form) || !apiEdited(incoming, form);
