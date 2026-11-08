// QR codes for the one-time key display. The reader below is a second, separate implementation
// of the format (read-only), so a placement or masking mistake in qr.js would show here.
import test from 'node:test';
import assert from 'node:assert/strict';
import { qrMatrix, qrPath } from '../qr.js';

// ---- GF(256), own tables
const EXP = new Array(512);
const LOG = new Array(256);
for (let i = 0, x = 1; i < 255; i++) {
  EXP[i] = x;
  LOG[x] = i;
  x <<= 1;
  if (x & 0x100) x ^= 0x11d;
}
for (let i = 255; i < 512; i++) EXP[i] = EXP[i - 255];
const mul = (a, b) => (a && b ? EXP[LOG[a] + LOG[b]] : 0);
// A codeword block is intact when it has zero syndromes at alpha^0..alpha^(ec-1).
function syndromesZero(block, ec) {
  for (let i = 0; i < ec; i++) {
    let s = 0;
    for (const c of block) s = mul(s, EXP[i]) ^ c;
    if (s) return false;
  }
  return true;
}

const BLOCKS_M = { 1: [[16, 10, 1]], 2: [[28, 16, 1]], 3: [[44, 26, 1]], 4: [[32, 18, 2]], 5: [[43, 24, 2]], 6: [[27, 16, 4]] };
const ALIGN = { 1: [], 2: [6, 18], 3: [6, 22], 4: [6, 26], 5: [6, 30], 6: [6, 34] };
const MASKS = [
  (x, y) => (x + y) % 2 === 0,
  (x, y) => y % 2 === 0,
  (x, y) => x % 3 === 0,
  (x, y) => (x + y) % 3 === 0,
  (x, y) => (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0,
  (x, y) => ((x * y) % 2) + ((x * y) % 3) === 0,
  (x, y) => (((x * y) % 2) + ((x * y) % 3)) % 2 === 0,
  (x, y) => (((x + y) % 2) + ((x * y) % 3)) % 2 === 0,
];

// Which modules carry data, from the standard's layout (not from qr.js).
function dataMap(v) {
  const size = 17 + 4 * v;
  const fn = Array.from({ length: size }, () => new Array(size).fill(false));
  const box = (x0, y0, w, h) => {
    for (let y = y0; y < y0 + h; y++) for (let x = x0; x < x0 + w; x++) if (y >= 0 && y < size && x >= 0 && x < size) fn[y][x] = true;
  };
  box(0, 0, 9, 9);
  box(size - 8, 0, 8, 9);
  box(0, size - 8, 9, 8);
  box(6, 0, 1, size);
  box(0, 6, size, 1);
  const a = ALIGN[v];
  for (const cy of a) for (const cx of a) if (!((cx === 6 && cy === 6) || (cx === 6 && cy === a.at(-1)) || (cx === a.at(-1) && cy === 6))) box(cx - 2, cy - 2, 5, 5);
  return fn;
}

function read(m) {
  const size = m.size;
  const v = (size - 17) / 4;
  const dark = (x, y) => m.get(x, y);
  // format info, both copies
  const bitsA = [];
  for (let i = 0; i <= 5; i++) bitsA[i] = dark(8, i);
  bitsA[6] = dark(8, 7);
  bitsA[7] = dark(8, 8);
  bitsA[8] = dark(7, 8);
  for (let i = 9; i < 15; i++) bitsA[i] = dark(14 - i, 8);
  const bitsB = [];
  for (let i = 0; i < 8; i++) bitsB[i] = dark(size - 1 - i, 8);
  for (let i = 8; i < 15; i++) bitsB[i] = dark(8, size - 15 + i);
  const num = (b) => b.reduce((acc, bit, i) => acc | (bit ? 1 << i : 0), 0);
  assert.equal(num(bitsA), num(bitsB), 'both format copies agree');
  const f = num(bitsA) ^ 0x5412;
  // valid BCH(15,5): the 15-bit word is divisible by the generator
  let rem = f;
  for (let i = 14; i >= 10; i--) if ((rem >> i) & 1) rem ^= 0x537 << (i - 10);
  assert.equal(rem, 0, 'format bits are a BCH codeword');
  assert.equal(f >> 13, 0, 'error correction level M');
  const mask = (f >> 10) & 7;
  assert.equal((f >> 10) & 0x18, 0);
  assert.ok(dark(8, size - 8), 'the always-dark module');
  // data, zigzag
  const fn = dataMap(v);
  const bits = [];
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5;
    const up = ((right + 1) & 2) === 0;
    for (let vert = 0; vert < size; vert++) {
      const y = up ? size - 1 - vert : vert;
      for (const x of [right, right - 1]) if (!fn[y][x]) bits.push(dark(x, y) !== MASKS[mask](x, y));
    }
  }
  const words = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) words.push(bits.slice(i, i + 8).reduce((acc, b) => (acc << 1) | (b ? 1 : 0), 0));
  const [[dpb, epb, nb]] = BLOCKS_M[v];
  assert.equal(words.length, nb * (dpb + epb), 'codeword count');
  const blocks = Array.from({ length: nb }, () => []);
  for (let i = 0; i < dpb; i++) for (let b = 0; b < nb; b++) blocks[b].push(words[i * nb + b]);
  const eo = dpb * nb;
  const ecs = Array.from({ length: nb }, () => []);
  for (let i = 0; i < epb; i++) for (let b = 0; b < nb; b++) ecs[b].push(words[eo + i * nb + b]);
  for (let b = 0; b < nb; b++) assert.ok(syndromesZero([...blocks[b], ...ecs[b]], epb), `block ${b} error correction`);
  const data = blocks.flat();
  const bitAt = (i) => (data[i >> 3] >> (7 - (i & 7))) & 1;
  const take = (at, n) => {
    let x = 0;
    for (let i = 0; i < n; i++) x = (x << 1) | bitAt(at + i);
    return x;
  };
  assert.equal(take(0, 4), 0b0100, 'byte mode');
  const n = take(4, 8);
  const out = Buffer.from(Array.from({ length: n }, (_, i) => take(12 + 8 * i, 8)));
  return out.toString('utf8');
}

test('a QR code reads back to the same text, with intact error correction, at every size', () => {
  for (const len of [1, 14, 15, 26, 27, 42, 43, 62, 63, 84, 85, 106]) {
    const text = Array.from({ length: len }, (_, i) => String.fromCharCode(33 + ((i * 7) % 90))).join('');
    const m = qrMatrix(text);
    assert.equal(m.size, 17 + 4 * ((m.size - 17) / 4));
    assert.equal(read(m), text, `length ${len}`);
  }
});

test('the version is the smallest that fits (M level)', () => {
  const sizeOf = (n) => qrMatrix('a'.repeat(n)).size;
  assert.equal(sizeOf(14), 21);
  assert.equal(sizeOf(15), 25);
  assert.equal(sizeOf(62), 33);
  assert.equal(sizeOf(63), 37);
  assert.equal(sizeOf(106), 41);
});

test('finder patterns sit in three corners and the fourth stays empty of one', () => {
  const m = qrMatrix('sode');
  const ring = (x0, y0) => m.get(x0, y0) && m.get(x0 + 6, y0 + 6) && !m.get(x0 + 1, y0 + 1) && m.get(x0 + 3, y0 + 3);
  assert.ok(ring(0, 0) && ring(m.size - 7, 0) && ring(0, m.size - 7));
  assert.ok(!ring(m.size - 7, m.size - 7));
});

test('too long a text is refused, and UTF-8 counts by bytes', () => {
  assert.throws(() => qrMatrix('x'.repeat(107)), /too long/);
  assert.equal(read(qrMatrix('é'.repeat(20))), 'é'.repeat(20));
});

test('qrPath draws every dark module once inside the quiet zone', () => {
  const m = qrMatrix('sode');
  const { d, box } = qrPath(m, 4);
  assert.equal(box, m.size + 8);
  const cells = [...d.matchAll(/M(\d+) (\d+)h1v1h-1z/g)].map((x) => [Number(x[1]) - 4, Number(x[2]) - 4]);
  let dark = 0;
  for (let y = 0; y < m.size; y++) for (let x = 0; x < m.size; x++) if (m.get(x, y)) dark++;
  assert.equal(cells.length, dark);
  assert.ok(cells.every(([x, y]) => m.get(x, y)));
});
