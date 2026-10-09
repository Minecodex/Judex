import { writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { build } from 'vite';
import react from '@vitejs/plugin-react';
import tailwind from '@tailwindcss/vite';

// The proposal uses the existing web dependencies, with no separate app install.
const webRoot = fileURLToPath(new URL('../', import.meta.url));
const result = await build({
  configFile: false,
  root: webRoot,
  plugins: [react(), tailwind()],
  logLevel: 'warn',
  define: { 'process.env.NODE_ENV': '"production"' },
  build: {
    write: false, cssCodeSplit: false, minify: true, cssMinify: true,
    target: 'es2022', assetsInlineLimit: Infinity,
    rolldownOptions: {
      input: resolve(webRoot, 'design-preview/App.tsx'),
      output: { format: 'iife', name: 'JudexDesignPreview' },
    },
  },
});
const output = (Array.isArray(result) ? result : [result]).flatMap((value) => value.output);
const entry = output.find((value) => value.type === 'chunk' && value.isEntry);
const css = output.filter((value) => value.type === 'asset' && value.fileName.endsWith('.css')).map((value) => String(value.source)).join('\n');
if (!entry || !css) throw new Error('The design preview bundle is missing JavaScript or CSS.');
const html = `<!doctype html>
<html lang="zh-CN" data-theme="light">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light dark">
<meta name="description" content="Judex 桌面端界面优化方案：登录、项目列表与三栏协作。独立预览，使用示例数据。">
<title>Judex · 界面设计预览</title>
<!-- Generated from web/design-preview. Open this file directly; no service or network is needed. -->
<style>${css.replace(/<\/style/gi, '<\\/style')}</style>
</head>
<body>
<div id="root"></div>
<noscript>请启用 JavaScript 以查看交互式界面预览。 Enable JavaScript to view the interactive preview.</noscript>
<script>${entry.code.replace(/<\/script/gi, '<\\/script')}</script>
</body>
</html>
`;
const target = resolve(webRoot, '../demo.html');
await writeFile(target, html, 'utf8');
console.log(`Created ${target} (${Math.round(Buffer.byteLength(html) / 1024)} KiB, self-contained).`);
