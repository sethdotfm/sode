// The recovery banner for a config with users and no Administrator (the hello's noAdministrator).

import { $, S, on } from './core.js';

on('hello', () => {
  $('no-admin').hidden = !S.noAdministrator;
});
