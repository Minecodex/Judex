import { writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { build } from 'vite';
import react from '@vitejs/plugin-react';
import tailwind from '@tailwindcss/vite';

const webRoot = fileURLToPath(new URL('../', import.meta.url));
const result = await build({
 configFile: false, root: webRoot, plugins: [react(), tailwind()], logLevel: 'warn',
 define: { 'process.env.NODE_ENV': '"production"' },
 build: { write: false, cssCodeSplit: false, minify: true, cssMinify: true, target: 'es2022', assetsInlineLimit: Infinity,
  rolldownOptions: { input: resolve(webRoot, 'v2-preview/App.tsx'), output: { format: 'iife', name: 'JudexV2Preview' } },
 },
});
const output = (Array.isArray(result) ? result : [result]).flatMap(value => value.output);
const entry = output.find(value => value.type === 'chunk' && value.isEntry);
const css = output.filter(value => value.type === 'asset' && value.fileName.endsWith('.css')).map(value => String(value.source)).join('\n');
if (!entry || !css) throw new Error('V2 preview build output is missing');
const html = `<!doctype html><html lang="zh-CN" data-theme="light"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="color-scheme" content="light dark"><meta name="description" content="Judex V2 群聊协作交互 Demo。示例数据与本地模拟 AI，不连接业务服务。"><title>Judex — 把想法，聊成可能</title><style>${css.replace(/<\/style/gi, '<\\/style')}</style></head><body><div id="root"></div><noscript>请启用 JavaScript 查看交互 Demo。</noscript><script>${entry.code.replace(/<\/script/gi, '<\\/script')}</script></body></html>`;
const target = resolve(webRoot, '../docs/plans/v2/demo.html');
await writeFile(target, html, 'utf8');
console.log(`Created ${target} (${Math.round(Buffer.byteLength(html) / 1024)} KiB), self-contained HeroUI V2 demo.`);
