// What a grid panel holds between taps (pure; Node tests load it).
// state: { lit: Set of destination numbers, src: the selected source or null }.
// On a panel with a Confirm cell a source tap only selects (one at a time)
// and Confirm sends; without one a source tap takes at once.

export const newState = () => ({ lit: new Set(), src: null });

export const hasConfirm = (grid) => !!grid?.cells.some((c) => c.kind === 'confirm');

export function tapDst(st, n) {
  const lit = new Set(st.lit);
  if (!lit.delete(n)) lit.add(n);
  return { ...st, lit };
}

// Selects the source, or unselects it when it is tapped again.
export const tapSrc = (st, n) => ({ ...st, src: st.src === n ? null : n });

// After Confirm sent: the source is let go, the destinations stay lit.
export const afterConfirm = (st) => ({ ...st, src: null });

export const clearAll = (st) => ({ ...st, lit: new Set(), src: null });

// Drops what the router no longer has.
export function prune(st, hasSrc, hasDst) {
  return {
    lit: new Set([...st.lit].filter(hasDst)),
    src: st.src != null && hasSrc(st.src) ? st.src : null,
  };
}

// Whether Confirm can be pressed, and if not what is missing.
export function confirmStatus(st) {
  const s = st.src != null, d = st.lit.size > 0;
  if (s && d) return { ready: true, text: '' };
  return { ready: false, text: s ? 'Light a destination' : d ? 'Pick a source' : 'Pick a source and a destination' };
}
