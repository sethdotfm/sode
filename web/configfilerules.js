// Pure parts of the server config file (file name, what a picked file may be); the server holds the real rules (internal/app/configfile.go).

import { dateStamp } from './exportfmt.js';

// maxConfigFile in Go.
export const MAX_BYTES = 2 << 20;

const clean = (s) =>
  String(s || '')
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^A-Za-z0-9._-]+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^[-.]+|[-.]+$/g, '')
    .slice(0, 40);

// YYYY-MM-DD-Sode-Server-Config-<Site>.yaml; a blank site is left out.
export function fileName({ now = new Date(), site = '' } = {}) {
  return [dateStamp(new Date(now)), 'Sode', 'Server', 'Config', clean(site)].filter(Boolean).join('-') + '.yaml';
}

export function fileProblem(bytes, text) {
  if (!bytes || !String(text).trim()) return 'That file is empty.';
  if (bytes > MAX_BYTES) return `That file is over ${MAX_BYTES >> 20} MiB; it isn't a Sode configuration.`;
  return '';
}

export const sizeText = (bytes) => (bytes < 1024 ? `${bytes} bytes` : `${Math.round(bytes / 1024)} KB`);
