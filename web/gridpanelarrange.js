// What the grid shows for a panel, its names and the search, and what the old view carries into it. Pure.
// A live update, a names poll, a Rights change and each search key all come through here (gridpanelrun.js rebuild).
import { gridAxes } from './gridpaneledges.js';
import { findMatches, shownOrder } from './gridpanelfind.js';
import { pinsOf } from './gridpanelwindow.js';
import { followCursor } from './gridpanelcursor.js';

// was is { find, picks, cur, axes }, axes being what the grid showed. full is gridAxes; axes is full less what a
// Filter hides. matches are best first (findMatches). The search is kept as it is; a gone current match falls to the first (currentMatch).
export function arrange(panel, namesReply, was) {
  const full = gridAxes(panel, namesReply);
  const off = namesReply.numberOffset || 0;
  const matches = {
    srcs: findMatches(full.srcs, namesReply.sources, was.find.srcs.text, off),
    dsts: findMatches(full.dsts, namesReply.destinations, was.find.dsts.text, off),
  };
  const axes = {
    srcs: shownOrder(full.srcs, pinsOf(panel.sources, full.srcs), was.find, 'srcs', matches.srcs),
    dsts: shownOrder(full.dsts, pinsOf(panel.destinations, full.dsts), was.find, 'dsts', matches.dsts),
  };
  // Picks on names that left the panel go; picks a Filter hides stay.
  const picks = { ...was.picks, srcs: was.picks.srcs.filter((n) => full.srcs.includes(n)), dsts: was.picks.dsts.filter((n) => full.dsts.includes(n)) };
  return { full, axes, matches, picks, cur: followCursor(was.cur, was.axes, axes) };
}
