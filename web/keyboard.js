// Is the on-screen keyboard up? iOS doesn't resize the layout viewport, so the visual viewport shrinks under it;
// pinch zoom (scale != 1) shrinks it too, which isn't a keyboard. Android resizes both, so this reads false.
export const kbdUp = (vv, innerHeight) => vv.scale === 1 && vv.height < innerHeight - 100;
