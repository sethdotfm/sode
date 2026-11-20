// A zip of text files, made in the page. Pure (node tests). Deflate comes from CompressionStream. No zip64, so each
// file and the whole zip stay under 4 GB; the Communication Log is far below that.

const CRC_TABLE = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  return t;
})();

function crc32(bytes) {
  let c = 0xffffffff;
  for (const b of bytes) c = CRC_TABLE[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

async function deflate(bytes) {
  const stream = new Blob([bytes]).stream().pipeThrough(new CompressionStream('deflate-raw'));
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

// The MS-DOS time and date that zip headers use, in local time.
const dosTime = (d) => (d.getHours() << 11) | (d.getMinutes() << 5) | (d.getSeconds() >> 1);
const dosDate = (d) => ((d.getFullYear() - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();

const UTF8_NAMES = 0x0800;
const DEFLATE = 8;

// files: [{ name, text }]. Gives a Blob of type application/zip.
export async function zipFiles(files, when = new Date()) {
  const enc = new TextEncoder();
  const parts = [];
  const central = [];
  let at = 0;
  for (const f of files) {
    const name = enc.encode(f.name);
    const raw = enc.encode(f.text);
    const packed = await deflate(raw);
    const crc = crc32(raw);
    // The fields from the version number to the name length are the same in both headers.
    const common = (v, p) => {
      v.setUint16(p, 20, true);
      v.setUint16(p + 2, UTF8_NAMES, true);
      v.setUint16(p + 4, DEFLATE, true);
      v.setUint16(p + 6, dosTime(when), true);
      v.setUint16(p + 8, dosDate(when), true);
      v.setUint32(p + 10, crc, true);
      v.setUint32(p + 14, packed.length, true);
      v.setUint32(p + 18, raw.length, true);
      v.setUint16(p + 22, name.length, true);
    };
    const local = new DataView(new ArrayBuffer(30));
    local.setUint32(0, 0x04034b50, true);
    common(local, 4);
    parts.push(local, name, packed);
    const dir = new DataView(new ArrayBuffer(46));
    dir.setUint32(0, 0x02014b50, true);
    dir.setUint16(4, 20, true);
    common(dir, 6);
    dir.setUint32(42, at, true);
    central.push(dir, name);
    at += 30 + name.length + packed.length;
  }
  const size = central.reduce((n, p) => n + p.byteLength, 0);
  const end = new DataView(new ArrayBuffer(22));
  end.setUint32(0, 0x06054b50, true);
  end.setUint16(8, files.length, true);
  end.setUint16(10, files.length, true);
  end.setUint32(12, size, true);
  end.setUint32(16, at, true);
  return new Blob([...parts, ...central, end], { type: 'application/zip' });
}
