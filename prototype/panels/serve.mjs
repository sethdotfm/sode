// PROTOTYPE: serves each variant on its own port; /shared and /vendor are common.
import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const web = path.resolve(here, '../../web')
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' }

function serve(port, variant) {
  http
    .createServer((req, res) => {
      let p = decodeURIComponent(new URL(req.url, 'http://x').pathname)
      if (p === '/') p = '/index.html'
      let file
      if (p.startsWith('/vendor/')) file = path.join(web, p)
      else if (p.startsWith('/shared/')) file = path.join(here, p)
      else file = path.join(here, variant, p)
      if (!file.startsWith(web) && !file.startsWith(here)) return res.writeHead(403).end()
      fs.readFile(file, (err, buf) => {
        if (err) return res.writeHead(404).end('not found')
        res.writeHead(200, { 'content-type': types[path.extname(file)] || 'application/octet-stream', 'cache-control': 'no-store' })
        res.end(buf)
      })
    })
    .listen(port, () => console.log(`variant ${variant.toUpperCase()}: http://localhost:${port}`))
}
serve(8101, 'a')
serve(8102, 'b')
serve(8103, 'c')
