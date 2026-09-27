import fs from 'node:fs';
import path from 'node:path';
import { load as yamlLoad, dump as yamlDump } from 'js-yaml';

// Contract generator entry point (run via `make generate` or `npm run gen:api`).
// Authoring format: api/openapi.header.yaml + api/components/*.yaml + api/paths/*.yaml.
// Output: single-file api/openapi.yaml (every cross-file $ref rewritten to
// #/components/...), synced to internal/gen/api/spec/openapi.yaml for go:embed.
const apiDir = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1'));

const load = (f) => yamlLoad(fs.readFileSync(path.join(apiDir, f), 'utf8'));
const header = fs.readFileSync(path.join(apiDir, 'openapi.header.yaml'), 'utf8');

const EXTERNAL_REF = /^(\.\.\/)?components\/[a-z]+\.yaml#\/components\/([a-zA-Z]+)\/([A-Za-z0-9_]+)$/;

// Rewrite refs on the parsed object graph so YAML dump style cannot break them.
function rewriteRefs(node) {
  if (Array.isArray(node)) {
    for (let i = 0; i < node.length; i++) node[i] = rewriteRefs(node[i]);
    return node;
  }
  if (node && typeof node === 'object') {
    for (const key of Object.keys(node)) {
      const value = node[key];
      if (typeof value === 'string') {
        const m = EXTERNAL_REF.exec(value);
        if (m) node[key] = `#/components/${m[2]}/${m[3]}`;
      } else {
        node[key] = rewriteRefs(value);
      }
    }
  }
  return node;
}

const components = { schemas: {}, parameters: {}, responses: {} };
const securitySchemes = {};
for (const f of fs.readdirSync(path.join(apiDir, 'components')).filter((f) => f.endsWith('.yaml')).sort()) {
  const doc = rewriteRefs(load('components/' + f));
  for (const key of Object.keys(components)) {
    Object.assign(components[key], doc.components?.[key] || {});
  }
  Object.assign(securitySchemes, doc.components?.securitySchemes || {});
}

const paths = {};
for (const f of fs.readdirSync(path.join(apiDir, 'paths')).filter((f) => f.endsWith('.yaml')).sort()) {
  const doc = rewriteRefs(load('paths/' + f));
  for (const [p, item] of Object.entries(doc.paths || {})) {
    if (paths[p]) {
      console.error('duplicate path', p);
      process.exit(1);
    }
    paths[p] = item;
  }
}

const body = yamlDump(
  { paths, components: { ...components, securitySchemes } },
  { lineWidth: 120, noRefs: true },
);
fs.writeFileSync(path.join(apiDir, 'openapi.yaml'), header.trimEnd() + '\n\n' + body);

const specDir = path.join(apiDir, '..', 'internal', 'gen', 'api', 'spec');
fs.rmSync(specDir, { recursive: true, force: true });
fs.mkdirSync(specDir, { recursive: true });
fs.copyFileSync(path.join(apiDir, 'openapi.yaml'), path.join(specDir, 'openapi.yaml'));

const METHODS = ['get', 'put', 'post', 'delete', 'patch', 'head', 'options', 'trace'];
const nOps = Object.values(paths).reduce((n, item) => n + METHODS.filter((m) => item[m]).length, 0);
console.log(`openapi.yaml assembled: ${Object.keys(paths).length} paths / ${nOps} operations`);
