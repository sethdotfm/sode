// The Grid panel's Levels box (levelbox.js). An embedded run has none, so the panel's levels setting stands in.
import { stageLevels, offeredLevels, boxFill } from './gridpanellevels.js';
import { parseLevelsIn } from './levels.js';
import { levelBox } from './levelbox.js';

// tally() and panel() are the run's last tally and panel; onChange runs on each edit of the box.
export function gridLevelBox(ui, tally, panel, onChange) {
  const boxNode = ui.levels;
  let boxKey = '';
  let filledKey = null;
  // The numbering the box text was typed in, so a change of Level numbering relabels it (levelbox.js reset).
  let typedIn = null;
  const boxSource = () => {
    const t = tally();
    return (
      t && {
        levels: t.levels,
        offered: offeredLevels(t, panel().rights),
        numbering: t.levelNumbering,
        defaultLevels: t.defaultLevels || '',
        prev: typedIn,
      }
    );
  };
  const lvl = boxNode
    ? levelBox(document.getElementById('panel-lvl'), document.getElementById('panel-lvl-pick'), document.getElementById('panel-lvl-all'), boxSource, {
        hold: 'pointerdown',
        onChange,
      })
    : null;
  if (boxNode) boxNode.hidden = false;

  return {
    // The picker is made again when the offered levels or the numbering change. The setting fills the box on load,
    // and again only when a live update changes the setting, so typed text survives other updates.
    sync() {
      const t = tally();
      const p = panel();
      if (!lvl || !t) return;
      const key = JSON.stringify([offeredLevels(t, p.rights).map((l) => l.level), t.levels.length, t.levelNumbering]);
      if (key !== boxKey) {
        boxKey = key;
        lvl.reset();
        typedIn = { mode: t.levelNumbering, max: t.levels.length };
      }
      const fk = JSON.stringify(p.settings?.levels ?? null);
      if (fk === filledKey) return;
      filledKey = fk;
      const fill = boxFill(p.settings, t, p.rights);
      if (fill) lvl.set(fill);
    },
    // The levels a press stages.
    levels() {
      const t = tally();
      const p = panel();
      if (lvl) return stageLevels(lvl.levels(), t, p.rights);
      const src = boxSource();
      return src ? stageLevels(boxFill(p.settings, t, p.rights) || parseLevelsIn('', src) || [], t, p.rights) : [];
    },
    // The box is empty, so a press stages the Default levels. A run with no box uses the panel's levels setting.
    empty() {
      const t = tally();
      const p = panel();
      return lvl ? lvl.empty() : !boxFill(p.settings, t, p.rights);
    },
    // The Grid popup fills the box from the main page's Level box. An empty list is an empty box: the default levels.
    fill(list) {
      if (!lvl) return;
      if (list?.length) return lvl.set(list);
      lvl.clear();
      onChange();
    },
    stop() {
      if (boxNode) boxNode.hidden = true;
    },
  };
}
