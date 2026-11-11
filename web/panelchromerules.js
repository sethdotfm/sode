// The panel page's chrome rules, with no DOM so they test directly (test/panelchrome.test.mjs).

// override is null until the Kiosk button is pressed; base is the panel setting or the #kiosk=1 link.
export const kioskNow = (override, base) => (override === null ? base : override);

export const headHidden = (kiosk, titleBar) => kiosk || !titleBar;

// Another handler that took Escape (a dialog, the PIN pad) has already prevented it.
export const leavesKiosk = (e, kiosk) => kiosk && e.key === 'Escape' && !e.defaultPrevented;

export const fullscreenWords = (on) => (on ? { icon: 'minimize', label: 'Leave full screen' } : { icon: 'maximize', label: 'Full screen' });

export const kioskWords = (on) => (on ? { icon: 'monitor-off', label: 'Leave kiosk mode' } : { icon: 'monitor', label: 'Kiosk mode' });
