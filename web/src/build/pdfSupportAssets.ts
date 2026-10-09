import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
import type {Plugin} from 'vite';

// PDF.js needs its packaged CMaps for non-embedded Chinese fonts, standard
// fonts and WASM decoders. Serve the same pinned vendor assets in dev/build.
export function pdfSupportAssets(): Plugin {
  const require = createRequire(import.meta.url), root = path.dirname(require.resolve('pdfjs-dist/package.json'));
  const version = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8')).version as string;
  const directories = {cmaps: 'cmaps', fonts: 'standard_fonts', wasm: 'wasm'};
  let destination = '';
  return {
    name: 'judex-pdf-support-assets',
    configResolved(config) {destination = path.resolve(config.root, config.build.outDir, 'assets/pdf-support', version);},
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const match = /^\/assets\/pdf-support\/([^/]+)\/(cmaps|fonts|wasm)\/(.*)$/.exec((req.url ?? '').split('?')[0]);
        if (!match) return next();
        if (match[1] !== version) return next();
        const directory = path.resolve(root, directories[match[2] as keyof typeof directories]);
        let file: string;
        try {file = path.resolve(directory, decodeURIComponent(match[3]));} catch {res.statusCode = 400;res.end();return;}
        if (!file.startsWith(directory + path.sep)) {res.statusCode = 403;res.end();return;}
        if (!fs.existsSync(file) || !fs.statSync(file).isFile()) return next();
        res.setHeader('Content-Type', file.endsWith('.wasm') ? 'application/wasm' : 'application/octet-stream');
        fs.createReadStream(file).pipe(res);
      });
    },
    closeBundle() {
      for (const [name, source] of Object.entries(directories)) fs.cpSync(path.join(root, source), path.join(destination, name), {recursive: true});
    },
  };
}
