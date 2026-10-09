import {writeFile} from "node:fs/promises";
import {fileURLToPath} from "node:url";
import {resolve} from "node:path";
import {build} from "vite";
import react from "@vitejs/plugin-react";
import tailwind from "@tailwindcss/vite";
const webRoot=fileURLToPath(new URL("../",import.meta.url));
const result=await build({
 configFile:false,root:webRoot,plugins:[react(),tailwind()],logLevel:"warn",define:{"process.env.NODE_ENV":'"production"'},
 build:{write:false,cssCodeSplit:false,minify:true,cssMinify:true,target:"es2022",assetsInlineLimit:Infinity,rolldownOptions:{input:resolve(webRoot,"cooperation-preview/App.tsx"),output:{format:"iife",name:"JudexCooperationPreview"}}},
});
const output=(Array.isArray(result)?result:[result]).flatMap(v=>v.output),entry=output.find(v=>v.type==="chunk"&&v.isEntry);
const css=output.filter(v=>v.type==="asset"&&v.fileName.endsWith(".css")).map(v=>String(v.source)).join("\n");
if(!entry||!css)throw new Error("Preview output missing");
const html=`<!doctype html><html lang="zh-CN" data-theme="light"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><meta name="description" content="Judex 项目协同、计划与任务讨论交互原型；只使用示例数据。"><title>Judex · 项目协同原型</title><style>${css.replace(/<\/style/gi,"<\\/style")}</style></head><body><div id="root"></div><noscript>请启用 JavaScript 查看交互原型。</noscript><script>${entry.code.replace(/<\/script/gi,"<\\/script")}</script></body></html>`;
const target=resolve(webRoot,"../demo.html");
await writeFile(target,html,"utf8");
console.log(`Created ${target}: ${Math.round(Buffer.byteLength(html)/1024)} KiB; self-contained HeroUI prototype`);
