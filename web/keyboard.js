// Is the on-screen keyboard up? iOS Safari doesn't resize the layout viewport
// for it, so the visual viewport (vv: { height, scale }) shrinks under the
// layout one (innerHeight). A pinch-zoomed page (scale ≠ 1) shrinks it too,
// which is not a keyboard; Android resizes the layout viewport itself (the
// two shrink together), so there it reads false and nothing is needed.
export const kbdUp = (vv, innerHeight) => vv.scale === 1 && vv.height < innerHeight - 100;
