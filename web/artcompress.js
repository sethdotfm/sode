// Uploaded artwork is shrunk in the browser before it is sent. The long edge is at most ART_EDGE px, PNG stays PNG
// for transparency and other photos become JPEG. GIF and SVG are sent as they are (a canvas would drop the
// animation or turn a vector into pixels). The server still caps a file at 1 MB (sidebarrules.js ART_MAX_BYTES).

export const ART_EDGE = 512;
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

// Returns the Blob to upload. Any failure (an unreadable image, no canvas) sends the original, which the server checks.
export async function compressImage(file, doc = document) {
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
