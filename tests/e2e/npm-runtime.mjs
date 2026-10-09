import fs from 'node:fs';
import path from 'node:path';

export function npmCliPath() {
  const nodeDirectory = path.dirname(process.execPath);
  const candidates = [
    process.env.npm_execpath,
    path.join(nodeDirectory, 'node_modules/npm/bin/npm-cli.js'),
    path.resolve(nodeDirectory, '../lib/node_modules/npm/bin/npm-cli.js'),
  ];
  const cli = candidates.find(candidate => candidate && fs.existsSync(candidate));
  if (!cli) throw new Error('npm CLI unavailable; install npm with Node.js or run through npm run test:e2e');
  return cli;
}
