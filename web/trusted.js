// Whether this connection is signed in as a Trusted user or higher and not locked (the last access event).

import { S, on } from './core.js';

let trustedUser = false;

export const trusted = () => trustedUser;

on('access', (m) => {
  trustedUser = !!m.trusted && !!m.user && !m.locked;
});
on('conn', () => {
  if (S.wsState !== 'up') trustedUser = false;
});
