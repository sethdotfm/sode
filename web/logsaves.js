// Words for a Settings saves row on /log (docs/agents/comm-log.md). Pure, no imports. The server
// already took out the values the reader's Role cannot see: those fields come with changed only.

export function show(v) {
  if (v === true) return 'On';
  if (v === false) return 'Off';
  if (v == null || v === '') return 'Blank';
  return JSON.stringify(v);
}

export const list = (vs) => vs.map((v) => (typeof v === 'string' ? v : JSON.stringify(v))).join(', ');

export function fieldText(f) {
  let what;
  if (f.changed) what = 'changed';
  else if (f.added || f.removed) {
    const parts = [];
    if (f.added?.length) parts.push(`added ${list(f.added)}`);
    if (f.removed?.length) parts.push(`removed ${list(f.removed)}`);
    what = parts.join('; ');
  } else what = `from ${show(f.old)} to ${show(f.new)}`;
  const text = `${f.section}: ${f.label}: ${what}`;
  return f.note ? `${text} (${f.note})` : text;
}

// Keep in step with CATEGORIES in exportrules.js (this module has no imports).
const IMPORT_CAT = {
  settings: 'Settings',
  routers: 'Routers',
  users: 'Users',
  presets: 'Presets',
  filterSets: 'Filter sets',
  panels: 'Panels',
  apiClients: 'API clients',
};

function importSummary(im) {
  const cats = (im.categories || []).map((c) => {
    if (c.refused) return `${IMPORT_CAT[c.category] || c.category} not imported`;
    const n = [
      [c.added, 'added'],
      [c.updated, 'updated'],
      [c.removed, 'removed'],
      [c.kept, 'kept'],
      [c.broken, 'Broken'],
    ].filter(([v]) => v);
    return `${IMPORT_CAT[c.category] || c.category} ${n.map(([v, w]) => `${v} ${w}`).join(', ') || 'no change'}`;
  });
  const mode = { add: 'Add import', replace: 'Replace import' }[im.mode] || 'Import';
  return `${mode}: ${cats.join('; ') || 'nothing'}`;
}

export function saveSummary(s) {
  const n = s.fields?.length || 0;
  const parts = [s.import ? importSummary(s.import) : `${n} ${n === 1 ? 'field' : 'fields'}`];
  if (s.critical) parts.push('Critical');
  if (s.approver) parts.push(`approved by ${s.approver}`);
  if (s.preset) parts.push(`preset ${s.preset}`);
  // Readers below Administrator get only the count (hiddenInterruptions).
  const stops = (s.interruptions?.length || 0) + (s.hiddenInterruptions || 0);
  if (stops) parts.push(`${stops} ${stops === 1 ? 'Interruption' : 'Interruptions'}`);
  return parts.join(', ');
}
