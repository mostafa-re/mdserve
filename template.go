package main

// pageTmpl is the single HTML shell: a left nav with a live filter box, the
// rendered body, optional CDN syntax-highlight + mermaid, and an optional
// live-reload client. Dark mode follows the OS via prefers-color-scheme.
const pageTmpl = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
{{if .CDN}}  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github.min.css" media="(prefers-color-scheme: light)">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github-dark.min.css" media="(prefers-color-scheme: dark)">
{{end}}  <style>
    :root{--bg:#fff;--fg:#24292f;--muted:#57606a;--side:#f6f8fa;--line:#d0d7de;--accent:#0969da;--code:#f6f8fa}
    @media (prefers-color-scheme:dark){:root{--bg:#0d1117;--fg:#e6edf3;--muted:#8b949e;--side:#161b22;--line:#30363d;--accent:#2f81f7;--code:#161b22}}
    *{box-sizing:border-box}
    body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;display:grid;grid-template-columns:300px 1fr;min-height:100vh;background:var(--bg);color:var(--fg)}
    nav{background:var(--side);padding:1rem;overflow-y:auto;border-right:1px solid var(--line);max-height:100vh;position:sticky;top:0}
    nav strong{display:block;margin-bottom:.5rem}
    nav input{width:100%;padding:.35rem .5rem;margin-bottom:.5rem;border:1px solid var(--line);border-radius:6px;background:var(--bg);color:var(--fg)}
    nav a{display:block;padding:.25rem .5rem;text-decoration:none;color:var(--muted);font-size:.86rem;border-radius:6px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
    nav a:hover{background:var(--line)}
    nav a.active{background:var(--accent);color:#fff}
    main{padding:2rem 3rem;max-width:54rem;line-height:1.55}
    a{color:var(--accent)}
    pre{background:var(--code);padding:1rem;overflow-x:auto;border-radius:8px;border:1px solid var(--line)}
    code{background:var(--code);padding:.1rem .35rem;border-radius:4px;font-size:.9em}
    pre code{background:none;padding:0;border:0}
    table{border-collapse:collapse;margin:1rem 0;display:block;overflow-x:auto}
    td,th{border:1px solid var(--line);padding:.4rem .8rem;text-align:left}
    h1,h2,h3{border-bottom:1px solid var(--line);padding-bottom:.3rem}
    blockquote{margin:0;padding:.2rem 1rem;color:var(--muted);border-left:4px solid var(--line)}
    img{max-width:100%}
  </style>
</head>
<body>
  <nav>
    <strong>{{.Title}}</strong>
    <input id="q" placeholder="filter docs…" autocomplete="off" spellcheck="false">
    <div id="nav">{{range .Nav}}
      <a href="{{.URL}}" data-n="{{.Name}}"{{if eq .Name $.Active}} class="active"{{end}}>{{.Name}}</a>{{end}}
    </div>
  </nav>
  <main>{{.Body}}</main>
  <script>
    const q=document.getElementById('q');
    if(q)q.addEventListener('input',()=>{const v=q.value.toLowerCase();
      document.querySelectorAll('#nav a').forEach(a=>{a.style.display=a.dataset.n.toLowerCase().includes(v)?'block':'none';});});
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
`
