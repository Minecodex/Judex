import http from "node:http";
import fs from "node:fs";
const port=Number(process.env.JUDEX_MATERIAL_PREVIEW_PORT??5197);
http.createServer((req,res)=>{
 if(!["/","/demo3.html"].includes((req.url??"").split("?")[0])){res.writeHead(404);res.end();return;}
 res.writeHead(200,{"Content-Type":"text/html; charset=utf-8","Cache-Control":"no-store"});res.end(fs.readFileSync("demo3.html"));
}).listen(port,"127.0.0.1",()=>console.log("Material prototype: http://127.0.0.1:"+port+"/demo3.html"));
