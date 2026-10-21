import http from 'node:http';
import { readFile } from 'node:fs/promises';
const port = Number(process.env.JUDEX_V2_PREVIEW_PORT || 5199);
http.createServer(async (req, res) => {
 if (!['/', '/demo.html'].includes((req.url || '').split('?')[0])) { res.writeHead(404); res.end(); return; }
 try { const html = await readFile(new URL('../../docs/plans/v2/demo.html', import.meta.url)); res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' }); res.end(html); }
 catch { res.writeHead(503); res.end('Build first: node web/v2-preview/build.mjs'); }
}).listen(port, '127.0.0.1', () => console.log(`Judex V2 demo: http://127.0.0.1:${port}/`));
