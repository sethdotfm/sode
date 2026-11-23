// Run setup again (Settings > Server; docs/agents/settings-saves.md). Pure, Node-tested. now: the Settings values
// with unsaved edits (readSettings plus each save source); presets: usersListResult.accessPresets.
import { presetGaps, presetTarget } from './accesspresetrules.js';

const GROUP_KEY = 'salvoGroup'; // the router's own salvo group, a Config JSON key

// Nothing records which preset was applied, so the pick is the one the values match, else none. The Guest's Use tie
// lines right counts too. Else a pick equal to the match stages nothing, and the Guest keeps a right the preset sets off.
const matching = (now, presets) => presets.find((p) => !presetGaps(presetTarget(p, []), now).length)?.key || '';

export function againAnswers(now, presets) {
  const was = matching(now, presets);
  return {
    again: true,
    was,
    brandText: now.brandText || '',
    brandBorder: now.brandBorder || '',
    name: '',
    pin: '',
    pin2: '',
    router: now.router || '',
    levels: String(now.levels ?? ''),
    levelNumbering: now.levelNumbering || '1',
    numberOffset: String(now.numberOffset ?? 1),
    fireMode: now.fireMode || 'sequential',
    group: String(now[GROUP_KEY] ?? 127),
    preset: was,
  };
}

// The fields Finish changes, as {key: value}: only those that differ from now. preset: the preset to apply, or null when the values already match the pick.
export function againStage(d, now, presets = []) {
  const want = {
    brandText: d.brandText.trim(),
    brandBorder: d.brandBorder,
    router: d.router.trim(),
    levels: +d.levels,
    levelNumbering: d.levelNumbering,
    numberOffset: +d.numberOffset,
    fireMode: d.fireMode,
  };
  // Sequential does not use the group, so a saved one stays.
  if (d.fireMode === 'native') want[GROUP_KEY] = +d.group;
  const fields = {};
  for (const [k, v] of Object.entries(want)) if (v !== now[k]) fields[k] = v;
  const preset = d.preset && d.preset !== d.was ? presets.find((p) => p.key === d.preset) || null : null;
  return { fields, preset };
}
