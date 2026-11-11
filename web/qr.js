// A small QR encoder for the one-time API key display: byte mode, error correction level M,
// versions 1-6 (up to 106 bytes), which is plenty for a key. Pure, no imports, so Node tests it.
// Layout follows ISO/IEC 18004; web/test/qr.test.mjs reads codes back with a separate reader.

// Per version: [data codewords per block, error codewords per block, blocks]. Level M.
const BLOCKS = { 1: [16, 10, 1], 2: [28, 16, 1], 3: [44, 26, 1], 4: [32, 18, 2], 5: [43, 24, 2], 6: [27, 16, 4] };
const ALIGN = { 1: [], 2: [6, 18], 3: [6, 22], 4: [6, 26], 5: [6, 30], 6: [6, 34] };
export const MAX_BYTES = 106;

// ---- Reed-Solomon over GF(256), polynomial 0x11d
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

function generator(n) {
  let g = [1];
  for (let i = 0; i < n; i++) {
    const next = new Array(g.length + 1).fill(0);
    g.forEach((c, j) => {
      next[j] ^= c;
      next[j + 1] ^= mul(c, EXP[i]);
    });
    g = next;
  }
  return g;
}

function remainder(data, n) {
  const g = generator(n);
  const r = new Array(n).fill(0);
  for (const b of data) {
    const f = b ^ r.shift();
    r.push(0);
    if (f) for (let i = 0; i < n; i++) r[i] ^= mul(g[i + 1], f);
  }
  return r;
}

// ---- codewords
function codewords(bytes, v) {
  const [dpb, epb, nb] = BLOCKS[v];
  const bits = [];
  const put = (value, n) => {
    for (let i = n - 1; i >= 0; i--) bits.push((value >> i) & 1);
  };
  put(0b0100, 4);
  put(bytes.length, 8);
  for (const b of bytes) put(b, 8);
  const cap = dpb * nb * 8;
  put(0, Math.min(4, cap - bits.length));
  while (bits.length % 8) bits.push(0);
  const data = [];
  for (let i = 0; i < bits.length; i += 8) data.push(parseInt(bits.slice(i, i + 8).join(''), 2));
  for (let pad = 0xec; data.length < dpb * nb; pad ^= 0xec ^ 0x11) data.push(pad);
  const blocks = Array.from({ length: nb }, (_, b) => data.slice(b * dpb, (b + 1) * dpb));
  const ecs = blocks.map((b) => remainder(b, epb));
  const out = [];
  for (let i = 0; i < dpb; i++) for (const b of blocks) out.push(b[i]);
  for (let i = 0; i < epb; i++) for (const e of ecs) out.push(e[i]);
  return out;
}

// ---- matrix
const MASKS = [
  (x, y) => (x + y) % 2 === 0,
  (_x, y) => y % 2 === 0,
  (x) => x % 3 === 0,
  (x, y) => (x + y) % 3 === 0,
  (x, y) => (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0,
  (x, y) => ((x * y) % 2) + ((x * y) % 3) === 0,
  (x, y) => (((x * y) % 2) + ((x * y) % 3)) % 2 === 0,
  (x, y) => (((x + y) % 2) + ((x * y) % 3)) % 2 === 0,
];

// Level M is 00 in the format word; BCH(15,5) with generator 0x537, then XOR 0x5412.
function formatBits(mask) {
  let rem = mask;
  for (let i = 0; i < 10; i++) rem = (rem << 1) ^ ((rem >> 9) * 0x537);
  return ((mask << 10) | rem) ^ 0x5412;
}

function build(words, v, mask) {
  const size = 17 + 4 * v;
  const mod = Array.from({ length: size }, () => new Array(size).fill(false));
  const fn = Array.from({ length: size }, () => new Array(size).fill(false));
  const set = (x, y, dark) => {
    if (x < 0 || y < 0 || x >= size || y >= size) return;
    mod[y][x] = dark;
    fn[y][x] = true;
  };
  for (let i = 0; i < size; i++) {
    set(6, i, i % 2 === 0);
    set(i, 6, i % 2 === 0);
  }
  const finder = (cx, cy) => {
    for (let dy = -4; dy <= 4; dy++)
      for (let dx = -4; dx <= 4; dx++) set(cx + dx, cy + dy, Math.max(Math.abs(dx), Math.abs(dy)) !== 4 && Math.max(Math.abs(dx), Math.abs(dy)) !== 2);
  };
  finder(3, 3);
  finder(size - 4, 3);
  finder(3, size - 4);
  const a = ALIGN[v];
  for (const cy of a) {
    for (const cx of a) {
      if ((cx === 6 && cy === 6) || (cx === 6 && cy === a.at(-1)) || (cx === a.at(-1) && cy === 6)) continue;
      for (let dy = -2; dy <= 2; dy++) for (let dx = -2; dx <= 2; dx++) set(cx + dx, cy + dy, Math.max(Math.abs(dx), Math.abs(dy)) !== 1);
    }
  }
  const drawFormat = (m) => {
    const bits = formatBits(m);
    const bit = (i) => ((bits >> i) & 1) === 1;
    for (let i = 0; i <= 5; i++) set(8, i, bit(i));
    set(8, 7, bit(6));
    set(8, 8, bit(7));
    set(7, 8, bit(8));
    for (let i = 9; i < 15; i++) set(14 - i, 8, bit(i));
    for (let i = 0; i < 8; i++) set(size - 1 - i, 8, bit(i));
    for (let i = 8; i < 15; i++) set(8, size - 15 + i, bit(i));
    set(8, size - 8, true);
  };
  drawFormat(0); // reserves the areas before the data goes in
  let i = 0;
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5;
    for (let vert = 0; vert < size; vert++) {
      const y = ((right + 1) & 2) === 0 ? size - 1 - vert : vert;
      for (const x of [right, right - 1]) {
        if (fn[y][x] || i >= words.length * 8) continue;
        mod[y][x] = ((words[i >> 3] >> (7 - (i & 7))) & 1) === 1;
        i++;
      }
    }
  }
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) if (!fn[y][x] && MASKS[mask](x, y)) mod[y][x] = !mod[y][x];
  drawFormat(mask);
  return { size, get: (x, y) => mod[y][x], mod };
}

// Chooses among the masks as the standard's four penalty rules do, roughly: the choice only
// affects how well a camera reads the code, never whether it is valid.
function penalty(m) {
  const { size, mod } = m;
  let p = 0;
  const lines = [...mod, ...mod.map((_, x) => mod.map((row) => row[x]))];
  for (const line of lines) {
    for (let i = 0; i < size; ) {
      let j = i;
      while (j < size && line[j] === line[i]) j++;
      if (j - i >= 5) p += 3 + (j - i - 5);
      i = j;
    }
    const s = line.map((b) => (b ? '1' : '0')).join('');
    for (const pat of ['000010111010', '010111010000']) {
      for (let at = s.indexOf(pat); at >= 0; at = s.indexOf(pat, at + 1)) p += 40;
    }
  }
  for (let y = 0; y + 1 < size; y++)
    for (let x = 0; x + 1 < size; x++) if (mod[y][x] === mod[y][x + 1] && mod[y][x] === mod[y + 1][x] && mod[y][x] === mod[y + 1][x + 1]) p += 3;
  const dark = mod.flat().filter(Boolean).length;
  p += (Math.ceil(Math.abs(dark * 20 - size * size * 10) / (size * size)) - 1) * 10;
  return p;
}

// {size, get(x, y)} with true for a dark module. Throws when text is over MAX_BYTES as UTF-8.
export function qrMatrix(text) {
  const bytes = [...new TextEncoder().encode(text)];
  const v = [1, 2, 3, 4, 5, 6].find((n) => bytes.length <= (BLOCKS[n][0] * BLOCKS[n][2] * 8 - 12) >> 3);
  if (!v) throw new Error(`too long for a QR code: ${bytes.length} bytes, at most ${MAX_BYTES}`);
  const words = codewords(bytes, v);
  let best = null;
  for (let mask = 0; mask < 8; mask++) {
    const m = build(words, v, mask);
    const score = penalty(m);
    if (!best || score < best.score) best = { m, score };
  }
  return best.m;
}

// One path of 1x1 squares plus a quiet zone of `quiet` modules, for an SVG with viewBox 0 0 box box.
export function qrPath(m, quiet = 4) {
  let d = '';
  for (let y = 0; y < m.size; y++) for (let x = 0; x < m.size; x++) if (m.get(x, y)) d += `M${x + quiet} ${y + quiet}h1v1h-1z`;
  return { d, box: m.size + quiet * 2 };
}
