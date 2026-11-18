// Which part of a Grid panel is drawn (gridpanelwindow.js): one axis at a time, in px, pinned items at the edge.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  axisSpan,
  spanMax,
  indexAt,
  slotAt,
  pageOf,
  placeAt,
  scrollFor,
  centreFor,
  pinsOf,
  pinsShown,
  wheelAcross,
  itemOf,
  slotOf,
  bandFor,
  bandCentre,
  wheelBand,
} from '../gridpanelwindow.js';

// 2500 sources of 70 px after a 220 px name column, in a 1060 px wide view: 12 to 13 columns show.
const ax = (over = {}) => ({ n: 2500, pins: 0, size: 70, head: 220, offset: 0, view: 1280, over: 0, ...over });

test('at the start, the span is the columns that show', () => {
  assert.deepEqual(axisSpan(ax()), { lo: 0, hi: 16 });
});

test('a scroll moves the span, and a part column counts', () => {
  assert.deepEqual(axisSpan(ax({ offset: 7000 })), { lo: 100, hi: 116 });
  assert.deepEqual(axisSpan(ax({ offset: 7035 })), { lo: 100, hi: 116 });
});

test('overscan adds items each side, clamped at the ends', () => {
  assert.deepEqual(axisSpan(ax({ over: 4 })), { lo: 0, hi: 20 });
  assert.deepEqual(axisSpan(ax({ offset: 7000, over: 4 })), { lo: 96, hi: 120 });
  const end = 2500 * 70 + 220 - 1280;
  assert.deepEqual(axisSpan(ax({ offset: end, over: 4 })), { lo: 2480, hi: 2500 });
});

test('pinned items are never in the span: they are drawn at the edge', () => {
  assert.deepEqual(axisSpan(ax({ pins: 3 })), { lo: 3, hi: 16 });
  // The pinned band covers what scrolls under it, so the first item that shows is past the scroll and the band.
  assert.deepEqual(axisSpan(ax({ pins: 3, offset: 7000 })), { lo: 103, hi: 116 });
});

test('a short axis is drawn whole and an empty one not at all', () => {
  assert.deepEqual(axisSpan(ax({ n: 5, over: 4 })), { lo: 0, hi: 5 });
  assert.deepEqual(axisSpan(ax({ n: 0 })), { lo: 0, hi: 0 });
  assert.deepEqual(axisSpan(ax({ n: 2, pins: 2 })), { lo: 2, hi: 2 });
});

test('indexAt finds the item under a point in the view', () => {
  assert.equal(indexAt(100, ax()), -1, 'the name bar');
  assert.equal(indexAt(220, ax()), 0);
  assert.equal(indexAt(289, ax({ offset: 7000 })), 100);
  assert.equal(indexAt(290, ax({ offset: 7000 })), 101);
  assert.equal(indexAt(250, ax({ pins: 2, offset: 7000 })), 0, 'a pinned item does not scroll');
  assert.equal(indexAt(370, ax({ pins: 2, offset: 7000 })), 102);
  assert.equal(indexAt(400, ax({ n: 2 })), -1, 'past the last item');
});

test('scrollFor keeps an item that shows where it is', () => {
  assert.equal(scrollFor(5, ax()), 0);
  assert.equal(scrollFor(105, ax({ offset: 7000 })), 7000);
  assert.equal(scrollFor(1, ax({ pins: 2, offset: 7000 })), 7000, 'pinned');
});

test('scrollFor brings an item into the view, clear of the name bar and the pinned band', () => {
  // Past the end: the item's far edge meets the view's far edge.
  assert.equal(scrollFor(100, ax()), 100 * 70 + 220 + 70 - 1280);
  // Before the start: the item's near edge meets the band's.
  assert.equal(scrollFor(100, ax({ offset: 9000 })), 7000);
  assert.equal(scrollFor(100, ax({ pins: 2, offset: 9000 })), 100 * 70 - 2 * 70);
});

test('centreFor puts an item in the middle of the view past the name bar and the pinned band', () => {
  // The item's middle (220 + 100 * 70 + 35) meets the middle of 220 to 1280.
  assert.equal(centreFor(100, ax()), 6505);
  assert.equal(centreFor(100, ax({ pins: 2, offset: 9000 })), 6435);
});

test('centreFor stays inside the scroll range and keeps a pinned item where it is', () => {
  assert.equal(centreFor(3, ax({ offset: 5000 })), 0);
  assert.equal(centreFor(2499, ax()), 220 + 2500 * 70 - 1280);
  assert.equal(centreFor(1, ax({ pins: 2, offset: 7000 })), 7000);
  assert.equal(centreFor(0, ax({ n: 3 })), 0, 'all items show');
});

test('pinsOf counts the pinned items at the front of display order', () => {
  const edge = { members: 'all', items: { 7: { pinned: true }, 3: { pinned: true }, 9: { pinned: false } } };
  assert.equal(pinsOf(edge, [3, 7, 0, 1, 2]), 2);
  assert.equal(pinsOf(undefined, [0, 1]), 0);
  assert.equal(pinsOf({ members: [5], items: { 5: { pinned: true } } }, [5]), 1);
});

test('spanMax is the longest span at any offset, so nodes made up front are never short while scrolling', () => {
  for (const over of [{}, { over: 4 }, { pins: 3, over: 2 }, { n: 9, over: 4 }, { view: 1000, size: 53 }]) {
    const a = ax(over);
    let most = 0;
    for (let offset = 0; offset < 4000; offset += 7) {
      const { lo, hi } = axisSpan({ ...a, offset });
      most = Math.max(most, hi - lo);
    }
    assert.ok(spanMax(a) >= most, `${JSON.stringify(over)}: ${spanMax(a)} < ${most}`);
    assert.ok(spanMax(a) <= most + 1, `${JSON.stringify(over)}: ${spanMax(a)} well over ${most}`);
  }
});

test('slotAt tells the name bar (-1) from past the last item (null)', () => {
  assert.equal(slotAt(100, ax()), -1);
  assert.equal(slotAt(220, ax()), 0);
  assert.equal(slotAt(290, ax({ offset: 7000 })), 101);
  assert.equal(slotAt(400, ax({ n: 2 })), null);
  assert.equal(slotAt(-1, ax()), null, 'before the view');
});

test('pageOf is the whole scrolling items one screen shows, at least one', () => {
  // 1280 - 220 = 1060 px: 15 whole items of 70.
  assert.equal(pageOf(ax()), 15);
  assert.equal(pageOf(ax({ pins: 3 })), 12);
  assert.equal(pageOf(ax({ view: 250 })), 1);
});

test('placeAt is an item start in content px; a pinned item and the bar follow the scroll', () => {
  assert.equal(placeAt(-1, ax({ offset: 7000 })), 7000);
  assert.equal(placeAt(1, ax({ pins: 2, offset: 7000 })), 7000 + 220 + 70);
  assert.equal(placeAt(100, ax({ pins: 2, offset: 7000 })), 220 + 100 * 70);
});

// The horizontal scroll bar of a 1000 by 600 px scroller at the window's top left, 36 px high, with 80 px crosspoints.
const bar = { left: 0, right: 980, top: 564, bottom: 600, line: 80, page: 980 };
const wheel = (o) => ({ clientX: 500, clientY: 580, deltaX: 0, deltaY: 100, deltaMode: 0, ctrlKey: false, ...o });

test('a vertical wheel over the horizontal scroll bar scrolls across, by lines or pages as the wheel gives them', () => {
  assert.equal(wheelAcross(wheel(), bar), 100);
  assert.equal(wheelAcross(wheel({ deltaY: -40 }), bar), -40);
  assert.equal(wheelAcross(wheel({ deltaY: 3, deltaMode: 1 }), bar), 240);
  assert.equal(wheelAcross(wheel({ deltaY: 1, deltaMode: 2 }), bar), 980);
});

test('a wheel past the bar, a mostly sideways wheel or a pinch zoom is left to the browser', () => {
  assert.equal(wheelAcross(wheel({ clientY: 563 }), bar), 0);
  assert.equal(wheelAcross(wheel({ clientX: 990 }), bar), 0);
  assert.equal(wheelAcross(wheel({ deltaX: 120 }), bar), 0);
  assert.equal(wheelAcross(wheel({ ctrlKey: true }), bar), 0);
  assert.equal(wheelAcross(wheel({ deltaY: 0 }), bar), 0);
});

test('pins draw as pinned only while they fill at most half the room past the name bar', () => {
  // 8 pins of 80 px under a 112 px name bar.
  const at = (view) => pinsShown({ pins: 8, size: 80, head: 112, view });
  assert.equal(at(2000), 8);
  assert.equal(at(540), 2);
  assert.equal(at(300), 1);
  assert.equal(at(150), 0);
  assert.equal(at(100), 0);
  assert.equal(pinsShown({ pins: 2, size: 80, head: 112, view: 2000 }), 2);
});

test('a band with more pins than slots maps slots to items: the band from its start, then the items past the pins', () => {
  const b = { pins: 10, slots: 4, at: 3 };
  assert.deepEqual(
    [-1, 0, 3, 4, 5].map((v) => itemOf(v, b)),
    [-1, 3, 6, 10, 11],
  );
  assert.deepEqual(
    [-1, 2, 3, 6, 7, 10, 11].map((k) => slotOf(k, b)),
    [-1, null, 0, 3, null, 4, 5],
  );
  const same = { pins: 3, slots: 3, at: 0 };
  assert.deepEqual(
    [0, 2, 3, 9].map((v) => itemOf(v, same)),
    [0, 2, 3, 9],
  );
  assert.deepEqual(
    [0, 2, 3, 9].map((k) => slotOf(k, same)),
    [0, 2, 3, 9],
  );
});

test('bandFor moves the band as little as it can to show a pin; bandCentre puts it in the middle', () => {
  const b = { pins: 10, slots: 4, at: 3 };
  assert.deepEqual(
    [0, 3, 6, 7, 9, 12].map((k) => bandFor(k, b)),
    [0, 3, 3, 4, 6, 3],
  );
  assert.deepEqual(
    [0, 5, 9, 12].map((k) => bandCentre(k, b)),
    [0, 4, 6, 3],
  );
});

test('a wheel over a band that scrolls moves the band; elsewhere, or with no more pins than slots, the grid scrolls', () => {
  const box = { left: 0, top: 100, right: 1000, bottom: 800, w: 200, h: 100, size: 70, line: 70, page: 600 };
  const bands = { srcs: { pins: 9, slots: 3, at: 0 }, dsts: { pins: 6, slots: 2, at: 0 } };
  const wheel = (x, y, dx, dy, more = {}) => wheelBand({ clientX: x, clientY: y, deltaX: dx, deltaY: dy, deltaMode: 0, ctrlKey: false, ...more }, box, bands);
  assert.deepEqual(wheel(500, 230, 0, 30), { kind: 'dsts', px: 30 });
  assert.deepEqual(wheel(500, 230, 0, 1, { deltaMode: 1 }), { kind: 'dsts', px: 70 });
  assert.equal(wheel(500, 400, 0, 30), null);
  assert.deepEqual(wheel(300, 600, 25, 0), { kind: 'srcs', px: 25 });
  assert.equal(wheel(300, 600, 0, 25), null);
  assert.equal(wheel(500, 230, 0, 30, { ctrlKey: true }), null);
  assert.equal(wheelBand({ clientX: 500, clientY: 230, deltaX: 0, deltaY: 30, deltaMode: 0 }, box, { ...bands, dsts: { pins: 2, slots: 2, at: 0 } }), null);
});
