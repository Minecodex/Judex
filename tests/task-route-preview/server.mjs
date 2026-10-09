import http from 'node:http';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
const file=fileURLToPath(new URL('../../demo4.html',import.meta.url)),port=Number(process.env.JUDEX_DEMO4_PORT??5396);
http.createServer(async(req,res)=>{if(!['/','/demo4.html'].includes((req.url??'').split('?')[0])){res.writeHead(404);res.end();return;}try{const html=await readFile(file);res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store'});res.end(html);}catch{res.writeHead(503);res.end('Demo4 has not been built yet');}}).listen(port,'127.0.0.1',()=>console.log(`Demo4: http://127.0.0.1:${port}/demo4.html`));
