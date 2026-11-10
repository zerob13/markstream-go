import mermaid from 'mermaid';
import katex from 'katex';
import {Infographic} from '@antv/infographic';
import {D2} from '@terrastruct/d2';
import DOMPurify from 'dompurify';
import 'katex/dist/katex.min.css';

const host=document.getElementById('content');
let serial=0;
const safeSVG=s=>DOMPurify.sanitize(s,{USE_PROFILES:{svg:true,svgFilters:true,html:true},ADD_TAGS:['style','foreignObject'],FORBID_TAGS:['script','iframe','object','embed','form','input','button','link','meta','base'],FORBID_ATTR:['onload','onclick']});
window.markstreamReady=true;
window.markstreamRender=async r=>{
 window.markstreamStage="start:"+r.kind;
 document.documentElement.style.colorScheme=r.dark?'dark':'light';
 document.body.style.background=r.dark?'#18181b':'#ffffff';
 host.style.cssText=`display:inline-block;max-width:${r.width||1024}px;font:${r.fontSize||16}px system-ui;color:${r.dark?'#fafafa':'#18181b'};padding:8px;box-sizing:border-box`;
 host.replaceChildren();host.style.width=(r.width||900)+"px";
 let svg='',plain='',depth=0;const config=r.config||{};
 if(r.kind==='math'||r.kind==='inline-math'){
  host.style.width='auto';host.style.whiteSpace='nowrap';host.style.padding=r.kind==='inline-math'?'0':'8px';
  katex.render(r.source,host,{...config,macros:{...(config.macros||{})},displayMode:r.kind==='math',throwOnError:true,strict:'error',trust:false,maxExpand:1000,maxSize:50,output:'htmlAndMathml'});
  plain=host.textContent;const strut=host.querySelector('.strut');if(strut)depth=Math.max(0,-parseFloat(getComputedStyle(strut).verticalAlign)||0);
 }else if(r.kind==='mermaid'){
  mermaid.initialize({...config,startOnLoad:false,securityLevel:'strict',theme:config.theme||(r.dark?'dark':'default'),suppressErrorRendering:true,fontFamily:'sans-serif',htmlLabels:false,maxTextSize:100000,flowchart:{htmlLabels:false},sequence:{useMaxWidth:false}});
  const out=await mermaid.render('markstream-'+(++serial),r.source);
  svg=safeSVG(out.svg);host.innerHTML=svg;plain=host.textContent;
 }else if(r.kind==='d2'){
  const d2=new D2();try{const compiled=await d2.compile(r.source,{...config,themeID:config.themeID??(r.dark?200:0),pad:config.pad??12});svg=safeSVG(await d2.render(compiled.diagram,compiled.renderOptions));host.innerHTML=svg;plain=host.textContent;}finally{d2.worker?.terminate?.()}
 }else if(r.kind==='infographic'){
  host.style.width=(r.width||900)+'px';
  const chart=new Infographic({...config,container:host,width:r.width||900,height:config.height||500,editable:false});
  try{await chart.render(r.source);await new Promise(resolve=>setTimeout(resolve,30));const el=host.querySelector('svg');if(!el)throw new Error('Infographic did not produce an SVG');svg=safeSVG(new XMLSerializer().serializeToString(el));plain=el.textContent;}finally{chart.destroy?.()}host.innerHTML=svg;
 }else if(r.kind==='svg'){svg=safeSVG(r.source);host.innerHTML=svg;plain=host.textContent;
 }else if(r.kind==='html'){
  host.style.width=(r.width||900)+'px';
  host.innerHTML=DOMPurify.sanitize(r.source,{FORBID_TAGS:['script','iframe','object','embed','form','input','button','style','link','meta','base'],FORBID_ATTR:['style','srcset'],ALLOW_DATA_ATTR:false});
  for(const el of host.querySelectorAll('[href],[src]')){for(const a of ['href','src']){const v=el.getAttribute(a);if(v&&!/^(https?:|mailto:|#|data:image\/(png|jpeg|gif|webp);base64,)/i.test(v))el.removeAttribute(a)}}
  plain=host.innerText;
 }else throw new Error('Unsupported rich kind: '+r.kind);
 window.markstreamStage="fonts"; await document.fonts.ready; window.markstreamStage="layout";
 const svgel=host.querySelector('svg');if(svgel){svgel.style.maxWidth='100%';svgel.style.height='auto';const vb=svgel.viewBox?.baseVal;if(vb?.width>0){svgel.style.width=Math.min(r.width||900,vb.width)+'px';}}
 await new Promise(resolve=>setTimeout(resolve,30));
 const b=host.getBoundingClientRect();if(b.width<1||b.height<1||b.width>4096||b.height>16000)throw new Error('Rendered dimensions exceed limits');
 const links=[...host.querySelectorAll('a[href]')].map(a=>{const q=a.getBoundingClientRect();return {url:a.getAttribute('href'),x:q.x-b.x,y:q.y-b.y,width:q.width,height:q.height,label:a.textContent}});
 window.markstreamStage="complete"; const accessible=host.cloneNode(true);accessible.querySelectorAll('style,script').forEach(el=>el.remove());plain=accessible.textContent;
 return {width:Math.ceil(b.width),height:Math.ceil(b.height),svg,plain,depth,links};
};
