// PROTOTYPE: serves the three editor variants on 8111 to 8116; /shared and /vendor are common.
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const here = path.dirname(fileURLToPath(import.meta.url))
const web = path.resolve(here, '../../web')
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' }
function serve(port, v) {
  http.createServer((req, res) => {
    let p = decodeURIComponent(new URL(req.url, 'http://x').pathname)
    if (p === '/') p = '/index.html'
    const file = p.startsWith('/vendor/') ? path.join(web, p) : p.startsWith('/shared/') ? path.join(here, p) : path.join(here, 'editor', v, p)
    if (!file.startsWith(web) && !file.startsWith(here)) return res.writeHead(403).end()
    fs.readFile(file, (err, buf) => err ? res.writeHead(404).end('not found') : (res.writeHead(200, { 'content-type': types[path.extname(file)] || 'application/octet-stream', 'cache-control': 'no-store' }), res.end(buf)))
  }).listen(port, () => console.log(`editor ${v.toUpperCase()}: http://localhost:${port}`))
}
serve(8111, 'a'); serve(8112, 'b'); serve(8113, 'c'); serve(8114, 'd'); serve(8115, 'e'); serve(8116, 'f')
