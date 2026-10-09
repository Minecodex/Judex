import http from "node:http";
import fs from "node:fs";
const port=Number(process.env.JUDEX_CO_PREVIEW_PORT??5195);
http.createServer((req,res)=>{
 if(!["/","/demo.html"].includes((req.url??"").split("?")[0])){res.writeHead(404);res.end();return;}
 res.writeHead(200,{"Content-Type":"text/html; charset=utf-8","Cache-Control":"no-store"});res.end(fs.readFileSync("demo.html"));
}).listen(port,"127.0.0.1",()=>console.log("Cooperation prototype: http://127.0.0.1:"+port+"/demo.html"));
