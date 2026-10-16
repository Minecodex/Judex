// Read-only capability imports from the bundled workspace runtime. No Office
// files are authored and no dependency is installed/downloaded by this probe.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const root=path.resolve('.cache/real-host/20261012-closure'),probe=path.join(root,'office-tools');
const bundle='C:/Users/23574/.cache/codex-runtimes/codex-primary-runtime/dependencies';
const bundledNode=path.join(bundle,'node/bin/node.exe'),modules=path.join(bundle,'node/node_modules');
const bundledPython=path.join(bundle,'python/python.exe');
if(!probe.startsWith(root+path.sep)||![bundledNode,modules,bundledPython].every(p=>fs.existsSync(p)))throw new Error('Verified bundled runtime paths required');
fs.mkdirSync(probe,{recursive:true});
const junction=path.join(probe,'node_modules');
if(fs.existsSync(junction)){if(fs.realpathSync(junction).toLowerCase()!==fs.realpathSync(modules).toLowerCase())throw new Error('Existing module path belongs to another runtime');}
else fs.symlinkSync(modules,junction,'junction');
let artifactToolImport=false,artifactToolError;
try{execFileSync(bundledNode,['--input-type','module','-e','await import("@oai/artifact-tool"); process.stdout.write("ARTIFACT_TOOL_IMPORT_OK");'],{cwd:probe,encoding:'utf8',windowsHide:true,timeout:30000});artifactToolImport=true;}
catch(error){artifactToolError=String(error.stderr??error.message).slice(0,800);}
const imports=JSON.parse(execFileSync(bundledPython,['-c','import importlib,json; result={}\nfor name in ["docx","openpyxl","pptx","PIL","pypdf","reportlab","fitz","pdfplumber","pypdfium2","cairosvg"]:\n try: importlib.import_module(name); result[name]="available"\n except Exception as e: result[name]="unavailable: "+type(e).__name__\nprint(json.dumps(result))'],{encoding:'utf8',windowsHide:true,timeout:30000}));
const result={observedAt:new Date().toISOString(),bundleVersion:'26.1009.11318',bundledNode,bundledPython,artifactToolImport,artifactToolError,imports,scope:'library import only, no authoring/rendering/recalculation claim; no network installation'};
fs.writeFileSync(path.join(root,'office-toolchain-probe.json'),JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify(result));
