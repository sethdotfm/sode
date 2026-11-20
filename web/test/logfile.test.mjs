import test from 'node:test';
import assert from 'node:assert/strict';
import { inflateRawSync, crc32 } from 'node:zlib';
import { LOG_COLS, csvCell, logCSV, statesCSV, minutesCSV } from '../logfile.js';
import { zipFiles } from '../zipfile.js';

const connect = {
  seq: 2,
  time: '2026-11-19T10:00:02Z',
  kind: 'connect',
  cmd: 2,
  level: 1,
  wire: 0,
  dst: 3,
  dstName: '=SUM(A1)',
  src: 4,
  srcName: 'CAM 5',
  why: 'take',
  by: '10.0.0.5',
  tries: 1,
  result: 'ack',
  hex: '02 00',
  ackMs: 1.5,
};
const save = {
  seq: 5,
  id: 1,
  time: '2026-11-19T10:00:04.5Z',
  kind: 'settings',
  cmd: -1,
  level: 0,
  wire: -1,
  dst: -1,
  src: -1,
  why: 'save',
  by: 'Ada',
  tries: 0,
  result: 'ok',
  hex: '',
  note: 'from the setup',
  save: {
    critical: true,
    fields: [
      { section: 'Branding', label: 'Logo text and page title', old: '', new: 'Studio B' },
      { section: 'Server', label: 'Trusted proxies', added: ['10.1.0.0/16'], removed: [] },
      { section: 'User Ana', label: 'PIN', changed: true },
      { section: '', label: 'Look', old: true, new: false, note: 'kept' },
    ],
    interruptions: ['Bo signed out'],
  },
};

const lines = (text) => text.split('\r\n');

test('the columns are those of the old Download CSV', () => {
  assert.equal(LOG_COLS.length, 18);
  assert.equal(lines(logCSV([], {}))[0], LOG_COLS.join(','));
  assert.equal(LOG_COLS[0], 'time');
  assert.equal(LOG_COLS[17], 'bytes');
});

test('a router message row', () => {
  const text = logCSV([connect], { off: 1, level: (n) => (n === 1 ? 'Video' : '') });
  assert.equal(lines(text)[1], "2026-11-19T10:00:02Z,2,Connect,0x02,Video,0,4,'=SUM(A1),5,CAM 5,Take,10.0.0.5,ack,1,,,1.5,02 00");
  assert.ok(text.endsWith('\r\n'));
});

test('an event row has no command byte and no numbers', () => {
  const e = {
    ...connect,
    kind: 'command',
    cmd: -1,
    level: 0,
    wire: -1,
    dst: -1,
    src: -1,
    dstName: '',
    srcName: '',
    why: 'stepIn',
    ackMs: undefined,
    hex: '',
    result: 'refused',
    tries: 0,
    err: 'no',
  };
  assert.equal(lines(logCSV([e], {}))[1], '2026-11-19T10:00:02Z,2,Command,,,,,,,,Step in,10.0.0.5,refused,0,no,,,');
});

test('a router salvo clear and a why with no label', () => {
  const e = { ...connect, kind: 'salvoGo', hex: '79 01 00', why: 'newThing' };
  const cells = lines(logCSV([e], {}))[1].split(',');
  assert.equal(cells[2], 'Router salvo clear');
  assert.equal(cells[10], 'newThing');
});

test('a Settings saves row lists its fields in the note', () => {
  const text = logCSV([save], {});
  const note = [
    'from the setup',
    '4 fields, Critical, 1 Interruption',
    'Branding: Logo text and page title: Blank → "Studio B"',
    'Server: Trusted proxies: added 10.1.0.0/16',
    'User Ana: PIN: changed',
    'Look: On → Off (kept)',
    'Interruption: Bo signed out',
  ].join('\n');
  assert.ok(text.includes(`,${csvCell(note)},`), text);
  assert.ok(text.includes('Settings save'));
});

test('the formula guard and quotes', () => {
  assert.equal(csvCell('=1+1'), "'=1+1");
  assert.equal(csvCell('-12.5'), '-12.5');
  assert.equal(csvCell('@x'), "'@x");
  assert.equal(csvCell('a,"b"'), '"a,""b"""');
  assert.equal(csvCell('a\nb'), '"a\nb"');
  assert.equal(csvCell(''), '');
});

test('the uptime files', () => {
  const t = Date.UTC(2026, 10, 19, 10, 0, 0);
  assert.equal(
    statesCSV([
      { t, s: 'up' },
      { t: t + 1500, s: 'down' },
      { t: t + 3000, s: 'off' },
    ]),
    'time,state,note\r\n2026-11-19T10:00:00.000Z,up,Router link up\r\n2026-11-19T10:00:01.500Z,down,Router link down or connecting\r\n2026-11-19T10:00:03.000Z,off,Sode not running\r\n',
  );
  assert.equal(minutesCSV([{ m: t / 60_000, bad: 2, slow: 0 }]), 'minute,bad,slow\r\n2026-11-19T10:00:00.000Z,2,0\r\n');
});

// Reads the zip as an unzip tool does: from the central directory.
function unzip(buf) {
  const v = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  const end = buf.byteLength - 22;
  assert.equal(v.getUint32(end, true), 0x06054b50);
  const n = v.getUint16(end + 10, true);
  let p = v.getUint32(end + 16, true);
  const out = {};
  for (let i = 0; i < n; i++) {
    assert.equal(v.getUint32(p, true), 0x02014b50);
    const crc = v.getUint32(p + 16, true);
    const packed = v.getUint32(p + 20, true);
    const size = v.getUint32(p + 24, true);
    const nameLen = v.getUint16(p + 28, true);
    const at = v.getUint32(p + 42, true);
    const name = new TextDecoder().decode(buf.subarray(p + 46, p + 46 + nameLen));
    assert.equal(v.getUint32(at, true), 0x04034b50);
    const data = at + 30 + v.getUint16(at + 26, true) + v.getUint16(at + 28, true);
    const body = inflateRawSync(buf.subarray(data, data + packed));
    assert.equal(body.length, size);
    assert.equal(crc32(body), crc);
    out[name] = body.toString('utf8');
    p += 46 + nameLen;
  }
  return out;
}

test('a zip that holds each file', async () => {
  const files = [
    { name: 'log.csv', text: 'time,seq\r\nÄ → 1\r\n'.repeat(500) },
    { name: 'uptime-states.csv', text: '' },
  ];
  const blob = await zipFiles(files, new Date(2026, 10, 19, 10, 30, 4));
  assert.equal(blob.type, 'application/zip');
  const got = unzip(new Uint8Array(await blob.arrayBuffer()));
  assert.deepEqual(got, { 'log.csv': files[0].text, 'uptime-states.csv': '' });
});
