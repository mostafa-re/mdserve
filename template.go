package main

// pageTmpl is the single HTML shell: a collapsible, resizable left nav rendered
// as a folder tree (SVG file/dir icons, open/closed dir state), the rendered
// body, a light/warm/dark theme switch, a back-to-top button, optional CDN
// syntax-highlight + mermaid, and an optional live-reload client. The "tree"
// sub-template recurses over treeNode children. No JS framework — one inline
// script wires the controls; preferences persist in localStorage.
const pageTmpl = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <script>try{var t=localStorage.getItem('mdserve-theme');document.documentElement.dataset.theme=t?t:(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');}catch(e){}</script>
{{if .CDN}}  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github.min.css" media="(prefers-color-scheme: light)">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github-dark.min.css" media="(prefers-color-scheme: dark)">
{{end}}  <style>
    :root{--side-w:300px;--bg:#fff;--fg:#24292f;--muted:#57606a;--side:#f6f8fa;--line:#d0d7de;--accent:#0969da;--code:#f6f8fa}
    @media (prefers-color-scheme:dark){:root:not([data-theme]){--bg:#0d1117;--fg:#e6edf3;--muted:#8b949e;--side:#161b22;--line:#30363d;--accent:#2f81f7;--code:#161b22}}
    :root[data-theme="light"]{--bg:#fff;--fg:#24292f;--muted:#57606a;--side:#f6f8fa;--line:#d0d7de;--accent:#0969da;--code:#f6f8fa}
    :root[data-theme="warm"]{--bg:#f8f1de;--fg:#4a3f33;--muted:#8a7a63;--side:#f1e7cd;--line:#e3d4ad;--accent:#b5651d;--code:#f1e7cd}
    :root[data-theme="dark"]{--bg:#0d1117;--fg:#e6edf3;--muted:#8b949e;--side:#161b22;--line:#30363d;--accent:#2f81f7;--code:#161b22}
    *{box-sizing:border-box}
    body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;display:grid;grid-template-columns:var(--side-w) 1fr;min-height:100vh;background:var(--bg);color:var(--fg)}
    nav{background:var(--side);padding:1rem;overflow-y:auto;overflow-x:hidden;border-right:1px solid var(--line);max-height:100vh;position:sticky;top:0;min-width:0}
    body[data-collapsed="1"]{--side-w:0}
    body[data-collapsed="1"] nav{padding:0;border-right:0}
    nav strong{display:block;margin-bottom:.5rem}
    nav input{width:100%;padding:.35rem .5rem;margin-bottom:.5rem;border:1px solid var(--line);border-radius:6px;background:var(--bg);color:var(--fg)}
    #nav ul{list-style:none;margin:0;padding:0}
    #nav ul ul{margin-left:.55rem;border-left:1px solid var(--line);padding-left:.2rem}
    #nav li{margin:.05rem 0}
    #nav a,#nav summary{display:flex;align-items:center;gap:.4rem;padding:.2rem .4rem;border-radius:6px;font-size:.86rem;color:var(--muted);text-decoration:none;cursor:pointer}
    #nav a:hover,#nav summary:hover{background:var(--line)}
    #nav a.active{background:var(--accent);color:#fff}
    #nav summary{list-style:none;font-weight:500;color:var(--fg)}
    #nav summary::-webkit-details-marker{display:none}
    #nav a span,#nav summary span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .ic{width:16px;height:16px;flex:0 0 auto;stroke:currentColor;fill:none;stroke-width:1.4;stroke-linecap:round;stroke-linejoin:round}
    summary .ic-open{display:none}
    details[open]>summary .ic-open{display:block}
    details[open]>summary .ic-closed{display:none}
    main{padding:2.2rem 3rem;max-width:54rem;line-height:1.55}
    a{color:var(--accent)}
    pre{background:var(--code);padding:1rem;overflow-x:auto;border-radius:8px;border:1px solid var(--line)}
    code{background:var(--code);padding:.1rem .35rem;border-radius:4px;font-size:.9em}
    pre code{background:none;padding:0;border:0}
    table{border-collapse:collapse;margin:1rem 0;display:block;overflow-x:auto}
    td,th{border:1px solid var(--line);padding:.4rem .8rem;text-align:left}
    h1,h2,h3{border-bottom:1px solid var(--line);padding-bottom:.3rem}
    blockquote{margin:0;padding:.2rem 1rem;color:var(--muted);border-left:4px solid var(--line)}
    img{max-width:100%}
    #bar{position:fixed;top:.6rem;right:.7rem;display:flex;gap:.3rem;z-index:30}
    #bar button,#top{display:inline-flex;align-items:center;justify-content:center;width:32px;height:32px;border:1px solid var(--line);background:var(--side);color:var(--muted);border-radius:8px;cursor:pointer;padding:0}
    #bar button:hover,#top:hover{color:var(--fg);border-color:var(--accent)}
    #bar button.on{color:var(--accent);border-color:var(--accent)}
    #resize{position:fixed;top:0;left:var(--side-w);width:8px;height:100vh;margin-left:-4px;cursor:col-resize;z-index:25}
    #resize:hover{background:var(--accent);opacity:.25}
    body[data-collapsed="1"] #resize{display:none}
    #top{position:fixed;right:.9rem;bottom:.9rem;width:40px;height:40px;border-radius:50%;opacity:0;pointer-events:none;transition:opacity .2s;box-shadow:0 2px 8px rgba(0,0,0,.25);z-index:30}
    #top .ic{width:18px;height:18px}
    #top.show{opacity:1;pointer-events:auto}
  </style>
</head>
<body>
  <svg width="0" height="0" style="position:absolute" aria-hidden="true">
    <symbol id="i-file" viewBox="0 0 16 16"><path d="M9 1.5H4a1 1 0 0 0-1 1v11a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V5.5z"/><path d="M9 1.5V5.5H13"/></symbol>
    <symbol id="i-folder" viewBox="0 0 16 16"><path d="M1.5 4.2c0-.6.4-1 1-1h3.3l1.4 1.6h6.3c.6 0 1 .4 1 1v6.4c0 .6-.4 1-1 1H2.5c-.6 0-1-.4-1-1z"/></symbol>
    <symbol id="i-folder-open" viewBox="0 0 16 16"><path d="M1.5 4.2c0-.6.4-1 1-1h3.3l1.4 1.6h6.3c.6 0 1 .4 1 1v1H1.5z"/><path d="M1.5 6.7l1.3 5.6c.1.4.5.7 1 .7h8.4c.5 0 .9-.3 1-.7l1.3-5.6z"/></symbol>
    <symbol id="i-up" viewBox="0 0 16 16"><path d="M8 13V3.5"/><path d="M3.5 8 8 3.5 12.5 8"/></symbol>
    <symbol id="i-sun" viewBox="0 0 16 16"><circle cx="8" cy="8" r="3.2"/><path d="M8 1v1.6M8 13.4V15M1 8h1.6M13.4 8H15M3 3l1.1 1.1M11.9 11.9 13 13M13 3l-1.1 1.1M4.1 11.9 3 13"/></symbol>
    <symbol id="i-warm" viewBox="0 0 16 16"><path d="M1.5 12.5h13"/><path d="M4.5 12.5a3.5 3.5 0 0 1 7 0"/><path d="M8 3.5v1.8M2.6 6.3l1.2 1.2M13.4 6.3l-1.2 1.2"/></symbol>
    <symbol id="i-moon" viewBox="0 0 16 16"><path d="M13 9.6A5.5 5.5 0 1 1 6.4 3a4.3 4.3 0 0 0 6.6 6.6z"/></symbol>
    <symbol id="i-sidebar" viewBox="0 0 16 16"><rect x="2" y="3" width="12" height="10" rx="1"/><path d="M6.5 3v10"/></symbol>
  </svg>
  <div id="bar">
    <button id="toggle" title="Toggle sidebar" aria-label="Toggle sidebar"><svg class="ic"><use href="#i-sidebar"/></svg></button>
    <button data-set-theme="light" title="Light" aria-label="Light theme"><svg class="ic"><use href="#i-sun"/></svg></button>
    <button data-set-theme="warm" title="Warm" aria-label="Warm theme"><svg class="ic"><use href="#i-warm"/></svg></button>
    <button data-set-theme="dark" title="Dark" aria-label="Dark theme"><svg class="ic"><use href="#i-moon"/></svg></button>
  </div>
  <nav>
    <strong>{{.Title}}</strong>
    <input id="q" placeholder="filter docs…" autocomplete="off" spellcheck="false">
    <div id="nav">{{template "tree" (dict "Nodes" .Tree "Active" .Active)}}</div>
  </nav>
  <div id="resize" title="Drag to resize"></div>
  <main>{{.Body}}</main>
  <button id="top" title="Back to top" aria-label="Back to top"><svg class="ic"><use href="#i-up"/></svg></button>
  <script>
  (function(){
    var root=document.documentElement,body=document.body;
    function setTheme(t){root.dataset.theme=t;try{localStorage.setItem('mdserve-theme',t)}catch(e){}
      document.querySelectorAll('[data-set-theme]').forEach(function(b){b.classList.toggle('on',b.dataset.setTheme===t)});}
    document.querySelectorAll('[data-set-theme]').forEach(function(b){b.addEventListener('click',function(){setTheme(b.dataset.setTheme)})});
    setTheme(root.dataset.theme||(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'));
    var tg=document.getElementById('toggle');
    function setCollapsed(c){body.dataset.collapsed=c?'1':'0';try{localStorage.setItem('mdserve-collapsed',c?'1':'0')}catch(e){}}
    if(tg)tg.addEventListener('click',function(){setCollapsed(body.dataset.collapsed!=='1')});
    try{if(localStorage.getItem('mdserve-collapsed')==='1')setCollapsed(true)}catch(e){}
    var rz=document.getElementById('resize'),dragging=false;
    try{var sw=localStorage.getItem('mdserve-side-w');if(sw)root.style.setProperty('--side-w',sw+'px')}catch(e){}
    if(rz){rz.addEventListener('pointerdown',function(e){dragging=true;try{rz.setPointerCapture(e.pointerId)}catch(_){}body.style.userSelect='none'});
      window.addEventListener('pointermove',function(e){if(!dragging)return;var x=Math.min(600,Math.max(160,e.clientX));root.style.setProperty('--side-w',x+'px')});
      window.addEventListener('pointerup',function(){if(!dragging)return;dragging=false;body.style.userSelect='';try{localStorage.setItem('mdserve-side-w',parseInt(getComputedStyle(root).getPropertyValue('--side-w'),10))}catch(e){}});}
    var top=document.getElementById('top');
    function onScroll(){if(top)top.classList.toggle('show',window.scrollY>300)}
    window.addEventListener('scroll',onScroll,{passive:true});onScroll();
    if(top)top.addEventListener('click',function(){window.scrollTo({top:0,behavior:'smooth'})});
    var q=document.getElementById('q');
    if(q)q.addEventListener('input',function(){
      var v=q.value.toLowerCase();
      document.querySelectorAll('#nav li.file').forEach(function(li){
        var a=li.querySelector('a');li.style.display=a&&a.dataset.n.toLowerCase().includes(v)?'':'none';});
      document.querySelectorAll('#nav li.dir').forEach(function(li){
        var vis=Array.prototype.some.call(li.querySelectorAll('li.file'),function(f){return f.style.display!=='none'});
        li.style.display=vis?'':'none';var d=li.querySelector('details');if(d&&v)d.open=true;});
    });
  })();
  </script>
{{if .CDN}}  <script src="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/highlight.min.js"></script>
  <script type="module">
    import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
    document.querySelectorAll('pre>code.language-mermaid').forEach(c=>{const d=document.createElement('div');d.className='mermaid';d.textContent=c.textContent;c.closest('pre').replaceWith(d);});
    mermaid.initialize({startOnLoad:true,theme:matchMedia('(prefers-color-scheme:dark)').matches?'dark':'default'});
    if(window.hljs)document.querySelectorAll('pre code:not(.language-mermaid)').forEach(b=>hljs.highlightElement(b));
  </script>
{{end}}{{if .LiveReload}}  <script>new EventSource('/__mdserve_reload').onmessage=()=>location.reload();</script>
{{end}}</body>
</html>
{{define "tree"}}<ul class="tree">{{$active := .Active}}{{range .Nodes}}{{if .IsDir}}
  <li class="dir"><details {{if hasPrefix $active (printf "%s/" .Rel)}}open{{end}}><summary><svg class="ic ic-closed"><use href="#i-folder"/></svg><svg class="ic ic-open"><use href="#i-folder-open"/></svg><span>{{.Name}}</span></summary>{{template "tree" (dict "Nodes" .Children "Active" $active)}}</details></li>{{else}}
  <li class="file"><a href="{{.URL}}" data-n="{{.Rel}}"{{if eq .Rel $active}} class="active"{{end}}><svg class="ic"><use href="#i-file"/></svg><span>{{.Name}}</span></a></li>{{end}}{{end}}
</ul>{{end}}
`
