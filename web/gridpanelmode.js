// Which parts of the Grid body (gridpanelrun.js) a run has. Pure.
// An adapter with its own confirm(routes, how) has no saved panel: it is the main page's Grid popup. Its staged routes go
// to that page's Confirm and take path. So the body has no confirm and no key list of its own. It also has no panel
// Undo, no Instant mode and no deck header. Every other adapter is a panel run (panelrun.js), on the panel page or in
// the Live Editor.

export function gridParts(ctx, panel) {
  const own = typeof ctx?.confirm !== 'function';
  return {
    // A panel id, a saved panel and the panel's own settings.
    panel: own,
    // The stage mode: a run with no saved panel is always Take mode with a confirm.
    mode: own ? panel?.settings?.mode : 'take',
    // The deck header is the panel page's only (gridpanelheadview.js).
    head: own && !ctx?.embedded && !!ctx?.ui?.name,
    // u asks GET /panels/{id}/undo. With no panel, u and Ctrl+Z go to the adapter's undo().
    panelUndo: own,
    // The panel page's #confirm and #keys. The main page has its own, which its own modules run.
    ownConfirm: own,
    keyList: own,
  };
}
