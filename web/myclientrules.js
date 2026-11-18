// My API clients, pure (no DOM) so Node tests them. Keep in step with ownClientScopes and
// SaveOwnClient in internal/app/ownclients.go.

import { SCOPES } from './apirules.js';

export const OWN_SCOPES = SCOPES.filter((s) => s.id !== 'log' && s.id !== 'panelsmanage');

const MAX_NAME = 24; // config.MaxUserName

export function clientNameProblem(name, mine) {
  const n = name.trim();
  if (!n) return 'Give the API client a name.';
  if ([...n].length > MAX_NAME) return `An API client name has at most ${MAX_NAME} characters.`;
  if (mine.some((m) => m.toLowerCase() === n.toLowerCase())) return `You already have an API client named ${n}.`;
  return '';
}

// No rights: the server copies yours for a new client.
export const saveReq = ({ name, scopes, id }) => ({ t: 'myClientSave', id, client: { name: name.trim(), scopes } });

export const keyReq = ({ client, name, pin, expires, id }) => ({ t: 'myClientKey', id, name: client, keyName: name.trim(), pin, expires });
