import {build} from 'esbuild';
import {readFile,writeFile,mkdir,readdir,rm} from 'node:fs/promises';
import {gzipSync} from 'node:zlib';
import path from 'node:path';
import {createRequire} from 'node:module';
const require=createRequire(import.meta.url),root=path.resolve(import.meta.dirname,'../../rich/assets'),out=path.resolve(import.meta.dirname,'.build');
await rm(out,{recursive:true,force:true});await mkdir(out,{recursive:true});await mkdir(root,{recursive:true});
await build({entryPoints:[path.join(import.meta.dirname,'entry.js')],outdir:out,bundle:true,splitting:true,format:'esm',minify:true,platform:'browser',conditions:['browser'],loader:{'.woff':'file','.woff2':'file','.ttf':'file'},assetNames:'[name]-[hash]',legalComments:'external',metafile:true});
await writeFile(path.join(out,'index.html'),'<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/entry.css"><style>body{margin:0}svg{display:block}pre{white-space:pre-wrap}img{max-width:100%}</style><div id="content"></div><script type="module" src="/entry.js"></script>');
await rm(root,{recursive:true,force:true});await mkdir(root,{recursive:true});
for(const f of await readdir(out)){const bytes=await readFile(path.join(out,f));await writeFile(path.join(root,f+'.gz'),gzipSync(bytes,{level:9,mtime:0}));}
// Preserve upstream license notices alongside bundled distributions.
const notices=['mermaid','katex','dompurify','@antv/infographic','@terrastruct/d2'];let licenses='Bundled renderer dependencies (exact versions in tools/renderer/package-lock.json)\n\n';
for(const pkg of notices){const dir=path.join(import.meta.dirname,'node_modules',pkg),files=await readdir(dir);let file=files.find(x=>/^license(?:\.md|\.txt)?$/i.test(x));licenses+='\n=== '+pkg+' ===\n'+(file?await readFile(path.join(dir,file),'utf8'):'See upstream source and package metadata.')+'\n';}
await writeFile(path.join(root,'THIRD_PARTY.txt'),licenses);
console.log('Generated', (await readdir(root)).length, 'embedded renderer assets');
