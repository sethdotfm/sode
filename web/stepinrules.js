// Step in rules (stepin.js, stepindialog.js): pure, no DOM.

// Step in needs another user with a PIN to step in as. users is the hello's list (len 0: no PIN).
export function stepInOffered({ locked, phone, stepped, user, users }) {
  if (locked || phone || stepped) return false;
  return (users || []).some((u) => u.len > 0 && u.name !== user);
}

// A link that leaves this page in this tab ends the step-in, so the page asks first.
export function stepsOut(href, here, target = '') {
  if (target && target !== '_self') return false;
  let to, from;
  try {
    to = new URL(href, here);
    from = new URL(here);
  } catch {
    return false;
  }
  return to.origin === from.origin && to.pathname !== from.pathname;
}
