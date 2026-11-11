// Uploaded artwork is shrunk in the browser before it is sent, so neither side passes ART_EDGE px (the server refuses
// more, panelArtSide in internal/app/panelstore.go). PNG stays PNG for transparency and JPEG stays JPEG. GIF and SVG
// are sent as they are (a canvas would drop the animation or turn a vector into pixels), so an oversize GIF is
// refused here. What is sent must fit ART_MAX_BYTES (sidebarrules.js).
import { artProblem, ART_TYPES, ART_MAX_BYTES } from './sidebarrules.js';

export const ART_EDGE = 1024;
const JPEG_QUALITY = 0.85;

// What to do with an image of this type and size: null leaves the file alone, else the canvas size and output type.
export function compressPlan(type, width, height) {
  if (type === 'image/gif' || type === 'image/svg+xml') return null;
  if (!(width > 0) || !(height > 0)) return null;
  const k = Math.min(1, ART_EDGE / Math.max(width, height));
  return {
    width: Math.max(1, Math.round(width * k)),
    height: Math.max(1, Math.round(height * k)),
    type: type === 'image/png' ? 'image/png' : 'image/jpeg',
    resized: k < 1,
  };
}

// A re-encoded file replaces the original only when it is smaller or the image had to shrink.
export const keepEncoded = (plan, before, after) => plan.resized || after < before;

// The logical screen size from a GIF's first 10 bytes, as Go's image.DecodeConfig reads it; null if not a GIF.
export function gifSize(head) {
  if (head.length < 10 || String.fromCharCode(...head.subarray(0, 4)) !== 'GIF8') return null;
  return { width: head[6] | (head[7] << 8), height: head[8] | (head[9] << 8) };
}

export function gifProblem(size, bytes) {
  if (bytes <= ART_MAX_BYTES && (!size || (size.width <= ART_EDGE && size.height <= ART_EDGE))) return '';
  return `A GIF is sent as it is, to keep its animation, so it must be at most ${ART_EDGE} by ${ART_EDGE} pixels and 2 MB. Resize it and upload again.`;
}

// Returns the Blob to upload. Any failure (an unreadable image, no canvas) sends the original, which the server checks.
export async function compressImage(file, doc = document) {
  if (!compressPlan(file.type, 1, 1)) return file;
  try {
    const bmp = await createImageBitmap(file);
    const plan = compressPlan(file.type, bmp.width, bmp.height);
    if (!plan) return file;
    const canvas = doc.createElement('canvas');
    canvas.width = plan.width;
    canvas.height = plan.height;
    canvas.getContext('2d').drawImage(bmp, 0, 0, plan.width, plan.height);
    bmp.close?.();
    const out = await new Promise((done) => canvas.toBlob(done, plan.type, JPEG_QUALITY));
    return out && keepEncoded(plan, file.size, out.size) ? out : file;
  } catch {
    return file;
  }
}

// What the editor uploads for a chosen file: { blob, problem: '' }, or { problem } when nothing may be sent.
export async function prepareArt(file, doc = document) {
  if (!ART_TYPES.includes(file.type)) return { problem: artProblem(file) };
  if (file.type === 'image/gif') {
    const problem = gifProblem(gifSize(new Uint8Array(await file.slice(0, 10).arrayBuffer())), file.size);
    return problem ? { problem } : { blob: file, problem };
  }
  const blob = await compressImage(file, doc);
  const problem = artProblem({ type: file.type, size: blob.size });
  return problem ? { problem } : { blob, problem };
}
