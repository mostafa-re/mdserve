package main

// pageTmpl is the single HTML shell: a clean top menubar split by a divider that
// lines up with the sidebar's right border — the mdserve logo + name sit over
// the sidebar; the controls sit over the content (left group: sidebar toggle,
// zoom stepper with an editable level, reset zoom; right group: theme/view
// toggle, then a right-aligned in-doc search). Below is a collapsible +
// resizable left nav rendered as a folder tree (SVG file/dir icons, open/closed
// dir state) that always fills the viewport height, the rendered body, a
// back-to-top button, optional CDN syntax-highlight + mermaid, and an optional
// live-reload client. The logo and favicon recolor to the active theme. The
// "tree" sub-template recurses over treeNode children. No JS framework — one
// inline script wires the controls; theme, zoom, sidebar width/state, and
// per-doc scroll position persist in localStorage.
const pageTmpl = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='7' fill='%232f81f7'/%3E%3Cpath d='M9 11h14M9 16h14M9 21h9' fill='none' stroke='white' stroke-width='2.4' stroke-linecap='round'/%3E%3C/svg%3E">
  <script>try{var d=document.documentElement,s=localStorage;d.dataset.theme=s.getItem('mdserve-theme')||'dark';var z=s.getItem('mdserve-zoom');if(z)d.style.setProperty('--zoom',z);var w=s.getItem('mdserve-side-w');if(w)d.style.setProperty('--side-w',w+'px');}catch(e){}</script>
{{if .CDN}}  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github.min.css" media="(prefers-color-scheme: light)">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/highlightjs/cdn-release@11/build/styles/github-dark.min.css" media="(prefers-color-scheme: dark)">
{{end}}  <style>
    :root{--side-w:300px;--bar-h:46px;--zoom:1;--bg:#fff;--fg:#24292f;--muted:#57606a;--side:#f6f8fa;--line:#d0d7de;--accent:#0969da;--code:#f6f8fa}
    :root[data-theme="light"]{--bg:#fff;--fg:#24292f;--muted:#57606a;--side:#f6f8fa;--line:#d0d7de;--accent:#0969da;--code:#f6f8fa}
    :root[data-theme="warm"]{--bg:#f4ece1;--fg:#4a4138;--muted:#877b6b;--side:#ece2d4;--line:#ddd1be;--accent:#b06a2c;--code:#ece2d4}
    :root[data-theme="dark"]{--bg:#0d1117;--fg:#e6edf3;--muted:#8b949e;--side:#161b22;--line:#30363d;--accent:#2f81f7;--code:#161b22}
    *{box-sizing:border-box}
    body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:var(--bg);color:var(--fg)}
    /* top menubar: brand over the sidebar (divider aligns with the sidebar border), controls over the content */
    #bar{position:sticky;top:0;z-index:30;display:flex;align-items:stretch;height:var(--bar-h);background:var(--side);border-bottom:1px solid var(--line)}
    .brand{flex:0 0 var(--side-w);width:var(--side-w);min-width:0;display:inline-flex;align-items:center;gap:.45rem;padding:0 1rem;font-weight:700;font-size:.95rem;color:var(--fg);text-decoration:none;border-right:1px solid var(--line);overflow:hidden;white-space:nowrap}
    .brand .logo{width:22px;height:22px;flex:0 0 auto}
    body[data-collapsed="1"] .brand{flex:0 0 auto;width:auto;border-right:0}
    .tools{flex:1 1 auto;min-width:0;display:flex;align-items:center;justify-content:space-between;gap:.5rem;padding:0 .7rem}
    .grp{display:flex;align-items:center;gap:.4rem;min-width:0}
    .grp>button{width:32px;height:32px;border:0;background:transparent;color:var(--muted);border-radius:7px;cursor:pointer;display:inline-flex;align-items:center;justify-content:center;padding:0;flex:0 0 auto}
    .grp>button:hover{background:var(--line);color:var(--fg)}
    #findbox{display:flex;align-items:center;gap:.2rem;height:32px;padding:0 .25rem 0 1.65rem;border:1px solid var(--line);background:var(--bg);border-radius:8px;position:relative}
    #findbox:focus-within{border-color:var(--accent)}
    #findbox .ibox-ic{position:absolute;left:.55rem;width:15px;height:15px;color:var(--muted);pointer-events:none}
    #findbox input{border:0;background:transparent;color:var(--fg);width:160px;outline:none;font-size:.85rem;padding:0}
    #findn{font-size:.72rem;color:var(--muted);min-width:2.2rem;text-align:right;white-space:nowrap}
    #findclear{width:22px;height:22px;border:0;background:transparent;color:var(--muted);cursor:pointer;display:none;align-items:center;justify-content:center;border-radius:5px;padding:0}
    #findclear:hover{color:var(--fg);background:var(--line)}
    #findclear .ic{width:13px;height:13px}
    .zoom{display:flex;align-items:center;height:32px;border:1px solid var(--line);background:var(--bg);border-radius:8px}
    .zoom button{width:26px;height:30px;border:0;background:transparent;color:var(--muted);cursor:pointer;font-size:16px;line-height:1;display:inline-flex;align-items:center;justify-content:center}
    .zoom button:hover{color:var(--fg)}
    .zoom input{width:3rem;border:0;background:transparent;color:var(--fg);text-align:center;font-size:.78rem;outline:none;padding:0}
    #theme .ic{display:none}
    :root[data-theme="light"] #theme .t-light{display:block}
    :root[data-theme="warm"] #theme .t-warm{display:block}
    :root[data-theme="dark"] #theme .t-dark{display:block}
    /* sidebar toggle swaps panel-close (when open) / panel-open (when collapsed) */
    #toggle .ic-popen{display:none}
    body[data-collapsed="1"] #toggle .ic-pclose{display:none}
    body[data-collapsed="1"] #toggle .ic-popen{display:block}
    .zoom button .ic{width:15px;height:15px}
    #layout{display:grid;grid-template-columns:var(--side-w) 1fr;align-items:start}
    nav{background:var(--side);padding:1rem;overflow-y:auto;overflow-x:hidden;border-right:1px solid var(--line);position:sticky;top:var(--bar-h);height:calc(100vh - var(--bar-h));min-width:0}
    body[data-collapsed="1"]{--side-w:0}
    body[data-collapsed="1"] nav{padding:0;border-right:0}
    .ibox{position:relative;display:flex;align-items:center}
    .ibox .ibox-ic{position:absolute;left:.5rem;width:15px;height:15px;color:var(--muted);pointer-events:none}
    .navfilter{margin-bottom:.5rem}
    .navfilter input{width:100%;padding:.35rem .5rem .35rem 1.7rem;border:1px solid var(--line);border-radius:6px;background:var(--bg);color:var(--fg)}
    #nav ul{list-style:none;margin:0;padding:0}
    #nav ul ul{margin-left:.55rem;border-left:1px solid var(--line);padding-left:.2rem}
    #nav li{margin:.05rem 0}
    #nav a,#nav summary{display:flex;align-items:center;gap:.4rem;padding:.2rem .4rem;border-radius:6px;font-size:.86rem;color:var(--muted);text-decoration:none;cursor:pointer}
    #nav a:hover,#nav summary:hover{background:var(--line)}
    #nav a.active{background:var(--accent);color:#fff}
    #nav summary{list-style:none;font-weight:500;color:var(--fg)}
    #nav summary::-webkit-details-marker{display:none}
    #nav a span,#nav summary span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .ic{width:16px;height:16px;flex:0 0 auto;fill:currentColor;stroke:none}
    summary .ic-open{display:none}
    details[open]>summary .ic-open{display:block}
    details[open]>summary .ic-closed{display:none}
    main{padding:2.2rem 3rem;max-width:54rem;line-height:1.55;font-size:calc(1rem * var(--zoom))}
    a{color:var(--accent)}
    pre{background:var(--code);padding:1rem;overflow-x:auto;border-radius:8px;border:1px solid var(--line)}
    code{background:var(--code);padding:.1rem .35rem;border-radius:4px;font-size:.9em}
    pre code{background:none;padding:0;border:0}
    table{border-collapse:collapse;margin:1rem 0;display:block;overflow-x:auto}
    td,th{border:1px solid var(--line);padding:.4rem .8rem;text-align:left}
    h1,h2,h3{border-bottom:1px solid var(--line);padding-bottom:.3rem}
    blockquote{margin:0;padding:.2rem 1rem;color:var(--muted);border-left:4px solid var(--line)}
    img{max-width:100%}
    mark.f{background:#fde047;color:#111;border-radius:2px;padding:0 .04em}
    mark.f.cur{box-shadow:0 0 0 2px var(--accent)}
    #resize{position:fixed;top:var(--bar-h);left:var(--side-w);width:8px;height:calc(100vh - var(--bar-h));margin-left:-4px;cursor:col-resize;z-index:25}
    #resize:hover{background:var(--accent);opacity:.25}
    body[data-collapsed="1"] #resize{display:none}
    #top{position:fixed;right:.9rem;bottom:.9rem;width:40px;height:40px;border-radius:50%;border:1px solid var(--line);background:var(--side);color:var(--muted);opacity:0;pointer-events:none;transition:opacity .2s;box-shadow:0 2px 8px rgba(0,0,0,.25);z-index:30;display:inline-flex;align-items:center;justify-content:center;cursor:pointer}
    #top:hover{color:var(--fg);border-color:var(--accent)}
    #top .ic{width:18px;height:18px}
    #top.show{opacity:1;pointer-events:auto}
  </style>
</head>
<body data-doc="{{.Active}}">
  <!-- Google Material Symbols (Rounded), inlined as a sprite. viewBox 0 -960 960 960, solid fills. -->
  <svg width="0" height="0" style="position:absolute" aria-hidden="true">
    <symbol id="i-file" viewBox="0 -960 960 960"><path d="M360-240h240q17 0 28.5-11.5T640-280q0-17-11.5-28.5T600-320H360q-17 0-28.5 11.5T320-280q0 17 11.5 28.5T360-240Zm0-160h240q17 0 28.5-11.5T640-440q0-17-11.5-28.5T600-480H360q-17 0-28.5 11.5T320-440q0 17 11.5 28.5T360-400ZM240-80q-33 0-56.5-23.5T160-160v-640q0-33 23.5-56.5T240-880h287q16 0 30.5 6t25.5 17l194 194q11 11 17 25.5t6 30.5v447q0 33-23.5 56.5T720-80H240Zm280-560v-160H240v640h480v-440H560q-17 0-28.5-11.5T520-640ZM240-800v200-200 640-640Z"/></symbol>
    <symbol id="i-folder" viewBox="0 -960 960 960"><path d="M160-160q-33 0-56.5-23.5T80-240v-480q0-33 23.5-56.5T160-800h207q16 0 30.5 6t25.5 17l57 57h320q33 0 56.5 23.5T880-640v400q0 33-23.5 56.5T800-160H160Zm0-80h640v-400H447l-80-80H160v480Zm0 0v-480 480Z"/></symbol>
    <symbol id="i-folder-open" viewBox="0 -960 960 960"><path d="M160-160q-33 0-56.5-23.5T80-240v-480q0-33 23.5-56.5T160-800h207q16 0 30.5 6t25.5 17l57 57h360q17 0 28.5 11.5T880-680q0 17-11.5 28.5T840-640H447l-80-80H160v480l79-263q8-26 29.5-41.5T316-560h516q41 0 64.5 32.5T909-457l-72 240q-8 26-29.5 41.5T760-160H160Zm84-80h516l72-240H316l-72 240Zm-84-262v-218 218Zm84 262 72-240-72 240Z"/></symbol>
    <symbol id="i-up" viewBox="0 -960 960 960"><path d="M440-647 244-451q-12 12-28 11.5T188-452q-11-12-11.5-28t11.5-28l264-264q6-6 13-8.5t15-2.5q8 0 15 2.5t13 8.5l264 264q11 11 11 27.5T772-452q-12 12-28.5 12T715-452L520-647v447q0 17-11.5 28.5T480-160q-17 0-28.5-11.5T440-200v-447Z"/></symbol>
    <symbol id="i-sun" viewBox="0 -960 960 960"><path d="M480-360q50 0 85-35t35-85q0-50-35-85t-85-35q-50 0-85 35t-35 85q0 50 35 85t85 35Zm0 80q-83 0-141.5-58.5T280-480q0-83 58.5-141.5T480-680q83 0 141.5 58.5T680-480q0 83-58.5 141.5T480-280ZM80-440q-17 0-28.5-11.5T40-480q0-17 11.5-28.5T80-520h80q17 0 28.5 11.5T200-480q0 17-11.5 28.5T160-440H80Zm720 0q-17 0-28.5-11.5T760-480q0-17 11.5-28.5T800-520h80q17 0 28.5 11.5T920-480q0 17-11.5 28.5T880-440h-80ZM480-760q-17 0-28.5-11.5T440-800v-80q0-17 11.5-28.5T480-920q17 0 28.5 11.5T520-880v80q0 17-11.5 28.5T480-760Zm0 720q-17 0-28.5-11.5T440-80v-80q0-17 11.5-28.5T480-200q17 0 28.5 11.5T520-160v80q0 17-11.5 28.5T480-40ZM226-678l-43-42q-12-11-11.5-28t11.5-29q12-12 29-12t28 12l42 43q11 12 11 28t-11 28q-11 12-27.5 11.5T226-678Zm494 495-42-43q-11-12-11-28.5t11-27.5q11-12 27.5-11.5T734-282l43 42q12 11 11.5 28T777-183q-12 12-29 12t-28-12Zm-42-495q-12-11-11.5-27.5T678-734l42-43q11-12 28-11.5t29 11.5q12 12 12 29t-12 28l-43 42q-12 11-28 11t-28-11ZM183-183q-12-12-12-29t12-28l43-42q12-11 28.5-11t27.5 11q12 11 11.5 27.5T282-226l-42 43q-11 12-28 11.5T183-183Zm297-297Z"/></symbol>
    <symbol id="i-warm" viewBox="0 -960 960 960"><path d="M792-670q11 11 11 28t-11 28l-29 29q-12 12-28.5 12T706-585q-12-12-11.5-28.5T707-642l29-29q12-11 28.5-10.5T792-670ZM120-160q-17 0-28.5-11.5T80-200q0-17 11.5-28.5T120-240h720q17 0 28.5 11.5T880-200q0 17-11.5 28.5T840-160H120Zm360-640q17 0 28.5 11.5T520-760v40q0 17-11.5 28.5T480-680q-17 0-28.5-11.5T440-720v-40q0-17 11.5-28.5T480-800ZM170-672q11-11 28-11t28 11l29 29q12 12 12 28.5T255-586q-12 11-29 11t-28-12l-29-29q-11-12-10.5-28.5T170-672Zm127 272h366q-23-54-72-87t-111-33q-62 0-111 33t-72 87Zm-97 80q0-117 81.5-198.5T480-600q117 0 198.5 81.5T760-320H200Zm280-80Z"/></symbol>
    <symbol id="i-moon" viewBox="0 -960 960 960"><path d="M480-120q-151 0-255.5-104.5T120-480q0-138 90-239.5T440-838q13-2 23 3.5t16 14.5q6 9 6.5 21t-7.5 23q-17 26-25.5 55t-8.5 61q0 90 63 153t153 63q31 0 61.5-9t54.5-25q11-7 22.5-6.5T819-479q10 5 15.5 15t3.5 24q-14 138-117.5 229T480-120Zm0-80q88 0 158-48.5T740-375q-20 5-40 8t-40 3q-123 0-209.5-86.5T364-660q0-20 3-40t8-40q-78 32-126.5 102T200-480q0 116 82 198t198 82Zm-10-270Z"/></symbol>
    <symbol id="i-panel-close" viewBox="0 -960 960 960"><path d="M660-368v-224q0-14-12-19t-22 5l-98 98q-12 12-12 28t12 28l98 98q10 10 22 5t12-19ZM200-120q-33 0-56.5-23.5T120-200v-560q0-33 23.5-56.5T200-840h560q33 0 56.5 23.5T840-760v560q0 33-23.5 56.5T760-120H200Zm120-80v-560H200v560h120Zm80 0h360v-560H400v560Zm-80 0H200h120Z"/></symbol>
    <symbol id="i-panel-open" viewBox="0 -960 960 960"><path d="M500-592v224q0 14 12 19t22-5l98-98q12-12 12-28t-12-28l-98-98q-10-10-22-5t-12 19ZM200-120q-33 0-56.5-23.5T120-200v-560q0-33 23.5-56.5T200-840h560q33 0 56.5 23.5T840-760v560q0 33-23.5 56.5T760-120H200Zm120-80v-560H200v560h120Zm80 0h360v-560H400v560Zm-80 0H200h120Z"/></symbol>
    <symbol id="i-search" viewBox="0 -960 960 960"><path d="M380-320q-109 0-184.5-75.5T120-580q0-109 75.5-184.5T380-840q109 0 184.5 75.5T640-580q0 44-14 83t-38 69l224 224q11 11 11 28t-11 28q-11 11-28 11t-28-11L532-372q-30 24-69 38t-83 14Zm0-80q75 0 127.5-52.5T560-580q0-75-52.5-127.5T380-760q-75 0-127.5 52.5T200-580q0 75 52.5 127.5T380-400Z"/></symbol>
    <symbol id="i-x" viewBox="0 -960 960 960"><path d="M480-424 284-228q-11 11-28 11t-28-11q-11-11-11-28t11-28l196-196-196-196q-11-11-11-28t11-28q11-11 28-11t28 11l196 196 196-196q11-11 28-11t28 11q11 11 11 28t-11 28L536-480l196 196q11 11 11 28t-11 28q-11 11-28 11t-28-11L480-424Z"/></symbol>
    <symbol id="i-filter" viewBox="0 -960 960 960"><path d="M440-160q-17 0-28.5-11.5T400-200v-240L168-736q-15-20-4.5-42t36.5-22h560q26 0 36.5 22t-4.5 42L560-440v240q0 17-11.5 28.5T520-160h-80Zm40-308 198-252H282l198 252Zm0 0Z"/></symbol>
    <symbol id="i-zoom-in" viewBox="0 -960 960 960"><path d="M340-540h-40q-17 0-28.5-11.5T260-580q0-17 11.5-28.5T300-620h40v-40q0-17 11.5-28.5T380-700q17 0 28.5 11.5T420-660v40h40q17 0 28.5 11.5T500-580q0 17-11.5 28.5T460-540h-40v40q0 17-11.5 28.5T380-460q-17 0-28.5-11.5T340-500v-40Zm40 220q-109 0-184.5-75.5T120-580q0-109 75.5-184.5T380-840q109 0 184.5 75.5T640-580q0 44-14 83t-38 69l224 224q11 11 11 28t-11 28q-11 11-28 11t-28-11L532-372q-30 24-69 38t-83 14Zm0-80q75 0 127.5-52.5T560-580q0-75-52.5-127.5T380-760q-75 0-127.5 52.5T200-580q0 75 52.5 127.5T380-400Z"/></symbol>
    <symbol id="i-zoom-out" viewBox="0 -960 960 960"><path d="M320-540q-17 0-28.5-11.5T280-580q0-17 11.5-28.5T320-620h120q17 0 28.5 11.5T480-580q0 17-11.5 28.5T440-540H320Zm60 220q-109 0-184.5-75.5T120-580q0-109 75.5-184.5T380-840q109 0 184.5 75.5T640-580q0 44-14 83t-38 69l224 224q11 11 11 28t-11 28q-11 11-28 11t-28-11L532-372q-30 24-69 38t-83 14Zm0-80q75 0 127.5-52.5T560-580q0-75-52.5-127.5T380-760q-75 0-127.5 52.5T200-580q0 75 52.5 127.5T380-400Z"/></symbol>
    <symbol id="i-zoom-reset" viewBox="0 -960 960 960"><path d="M393-132q-103-29-168-113.5T160-440q0-57 19-108.5t54-94.5q11-12 27-12.5t29 12.5q11 11 11.5 27T290-586q-24 31-37 68t-13 78q0 81 47.5 144.5T410-209q13 4 21.5 15t8.5 24q0 20-14 31.5t-33 6.5Zm174 0q-19 5-33-7t-14-32q0-12 8.5-23t21.5-15q75-24 122.5-87T720-440q0-100-70-170t-170-70h-3l16 16q11 11 11 28t-11 28q-11 11-28 11t-28-11l-84-84q-6-6-8.5-13t-2.5-15q0-8 2.5-15t8.5-13l84-84q11-11 28-11t28 11q11 11 11 28t-11 28l-16 16h3q134 0 227 93t93 227q0 109-65 194T567-132Z"/></symbol>
  </svg>
  <header id="bar">
    <a class="brand" href="/" title="mdserve"><svg class="logo" viewBox="0 0 32 32"><rect width="32" height="32" rx="7" fill="var(--accent)"/><path d="M9 11h14M9 16h14M9 21h9" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/></svg>mdserve</a>
    <div class="tools">
      <div class="grp">
        <button id="toggle" title="Toggle sidebar" aria-label="Toggle sidebar"><svg class="ic ic-pclose"><use href="#i-panel-close"/></svg><svg class="ic ic-popen"><use href="#i-panel-open"/></svg></button>
        <div class="zoom"><button id="zoomout" title="Zoom out" aria-label="Zoom out"><svg class="ic"><use href="#i-zoom-out"/></svg></button><input id="zoomval" title="Zoom level (type a %)" aria-label="Zoom level" value="100%"><button id="zoomin" title="Zoom in" aria-label="Zoom in"><svg class="ic"><use href="#i-zoom-in"/></svg></button></div>
        <button id="zoomreset" title="Reset zoom" aria-label="Reset zoom"><svg class="ic"><use href="#i-zoom-reset"/></svg></button>
      </div>
      <div class="grp">
        <button id="theme" title="Theme (dark / light / warm)" aria-label="Switch theme"><svg class="ic t-dark"><use href="#i-moon"/></svg><svg class="ic t-light"><use href="#i-sun"/></svg><svg class="ic t-warm"><use href="#i-warm"/></svg></button>
        <div id="findbox"><svg class="ic ibox-ic"><use href="#i-search"/></svg><input id="find" type="text" placeholder="search doc…" autocomplete="off" spellcheck="false"><span id="findn"></span><button id="findclear" title="Clear search" aria-label="Clear search"><svg class="ic"><use href="#i-x"/></svg></button></div>
      </div>
    </div>
  </header>
  <div id="layout">
    <nav>
      <div class="ibox navfilter"><svg class="ic ibox-ic"><use href="#i-filter"/></svg><input id="q" placeholder="filter docs…" autocomplete="off" spellcheck="false"></div>
      <div id="nav">{{template "tree" (dict "Nodes" .Tree "Active" .Active)}}</div>
    </nav>
    <div id="resize" title="Drag to resize"></div>
    <main>{{.Body}}</main>
  </div>
  <button id="top" title="Back to top" aria-label="Back to top"><svg class="ic"><use href="#i-up"/></svg></button>
  <script>
  (function(){
    var root=document.documentElement,body=document.body,main=document.querySelector('main');
    var doc=body.dataset.doc||'';
    function save(k,v){try{localStorage.setItem(k,v)}catch(e){}}
    // favicon recolors to the active theme's accent (same mark as the menubar logo)
    var accents={dark:'#2f81f7',light:'#0969da',warm:'#b06a2c'};
    var favLink=document.querySelector('link[rel="icon"]');
    function favSvg(c){return "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='7' fill='"+encodeURIComponent(c)+"'/%3E%3Cpath d='M9 11h14M9 16h14M9 21h9' fill='none' stroke='white' stroke-width='2.4' stroke-linecap='round'/%3E%3C/svg%3E"}
    function setFavicon(t){if(favLink)favLink.href=favSvg(accents[t]||accents.dark)}
    // theme — one button cycles dark -> light -> warm (the logo recolors via --accent)
    var order=['dark','light','warm'];
    function setTheme(t){root.dataset.theme=t;save('mdserve-theme',t);setFavicon(t)}
    var themeBtn=document.getElementById('theme');
    if(themeBtn)themeBtn.addEventListener('click',function(){var i=order.indexOf(root.dataset.theme);setTheme(order[(i+1)%order.length])});
    setTheme(root.dataset.theme||'dark');
    // zoom — steppers, an editable level indicator, and a reset
    var zi=document.getElementById('zoomin'),zo=document.getElementById('zoomout'),zr=document.getElementById('zoomreset'),zv=document.getElementById('zoomval');
    function curZoom(){return parseFloat(getComputedStyle(root).getPropertyValue('--zoom'))||1}
    function renderZoom(){if(zv&&document.activeElement!==zv)zv.value=Math.round(curZoom()*100)+'%'}
    function setZoom(z){z=Math.min(2,Math.max(.5,Math.round(z*100)/100));root.style.setProperty('--zoom',z);save('mdserve-zoom',z);renderZoom()}
    if(zi)zi.addEventListener('click',function(){setZoom(curZoom()+.1)});
    if(zo)zo.addEventListener('click',function(){setZoom(curZoom()-.1)});
    if(zr)zr.addEventListener('click',function(){setZoom(1)});
    if(zv){zv.addEventListener('change',function(){var n=parseInt(zv.value,10);if(n)setZoom(n/100);else renderZoom()});
      zv.addEventListener('keydown',function(e){if(e.key==='Enter')zv.blur()});
      zv.addEventListener('focus',function(){zv.select()});
      zv.addEventListener('blur',renderZoom)}
    renderZoom();
    // sidebar collapse
    var tg=document.getElementById('toggle');
    function setCollapsed(c){body.dataset.collapsed=c?'1':'0';save('mdserve-collapsed',c?'1':'0')}
    if(tg)tg.addEventListener('click',function(){setCollapsed(body.dataset.collapsed!=='1')});
    try{if(localStorage.getItem('mdserve-collapsed')==='1')setCollapsed(true)}catch(e){}
    // sidebar resize
    var rz=document.getElementById('resize'),dragging=false;
    if(rz){rz.addEventListener('pointerdown',function(e){dragging=true;try{rz.setPointerCapture(e.pointerId)}catch(_){}body.style.userSelect='none'});
      window.addEventListener('pointermove',function(e){if(!dragging)return;var x=Math.min(600,Math.max(160,e.clientX));root.style.setProperty('--side-w',x+'px')});
      window.addEventListener('pointerup',function(){if(!dragging)return;dragging=false;body.style.userSelect='';save('mdserve-side-w',parseInt(getComputedStyle(root).getPropertyValue('--side-w'),10))});}
    // back-to-top + per-doc scroll memory
    var top=document.getElementById('top'),tick=false;
    function onScroll(){if(top)top.classList.toggle('show',window.scrollY>300);
      if(!tick){tick=true;requestAnimationFrame(function(){tick=false;if(doc)save('mdserve-pos:'+doc,window.scrollY)})}}
    window.addEventListener('scroll',onScroll,{passive:true});
    window.addEventListener('pagehide',function(){if(doc)save('mdserve-pos:'+doc,window.scrollY)});
    document.addEventListener('visibilitychange',function(){if(document.visibilityState==='hidden'&&doc)save('mdserve-pos:'+doc,window.scrollY)});
    if(top)top.addEventListener('click',function(){window.scrollTo({top:0,behavior:'smooth'})});
    function restore(){if(!doc)return;try{var y=localStorage.getItem('mdserve-pos:'+doc);if(y)window.scrollTo(0,parseInt(y,10))}catch(e){}}
    restore();window.addEventListener('load',restore);onScroll();
    // file filter (tree)
    var q=document.getElementById('q');
    if(q)q.addEventListener('input',function(){
      var v=q.value.toLowerCase();
      document.querySelectorAll('#nav li.file').forEach(function(li){var a=li.querySelector('a');li.style.display=a&&a.dataset.n.toLowerCase().includes(v)?'':'none'});
      document.querySelectorAll('#nav li.dir').forEach(function(li){
        var vis=Array.prototype.some.call(li.querySelectorAll('li.file'),function(f){return f.style.display!=='none'});
        li.style.display=vis?'':'none';var dt=li.querySelector('details');if(dt&&v)dt.open=true});
    });
    // in-doc search
    var marks=[],cur=-1,findn=document.getElementById('findn'),findclear=document.getElementById('findclear');
    function clearFind(){marks.forEach(function(m){var t=document.createTextNode(m.textContent);m.parentNode.replaceChild(t,m)});marks=[];cur=-1;if(main)main.normalize();if(findn)findn.textContent=''}
    function focusMark(){marks.forEach(function(m){m.classList.remove('cur')});if(cur<0||!marks[cur])return;marks[cur].classList.add('cur');marks[cur].scrollIntoView({block:'center',behavior:'smooth'});if(findn)findn.textContent=(cur+1)+'/'+marks.length}
    function step(d){if(!marks.length)return;cur=(cur+d+marks.length)%marks.length;focusMark()}
    function doFind(query){
      clearFind();if(!query||!main)return;
      var low=query.toLowerCase(),walker=document.createTreeWalker(main,NodeFilter.SHOW_TEXT),targets=[],n;
      while(n=walker.nextNode()){var p=n.parentNode;if(!p)continue;if(p.nodeName==='MARK'||p.nodeName==='SCRIPT'||p.nodeName==='STYLE')continue;if(n.nodeValue.toLowerCase().indexOf(low)!==-1)targets.push(n)}
      targets.forEach(function(node){var text=node.nodeValue,lt=text.toLowerCase(),idx=0,pos,frag=document.createDocumentFragment();
        while((pos=lt.indexOf(low,idx))!==-1){if(pos>idx)frag.appendChild(document.createTextNode(text.slice(idx,pos)));
          var mk=document.createElement('mark');mk.className='f';mk.textContent=text.slice(pos,pos+query.length);frag.appendChild(mk);marks.push(mk);idx=pos+query.length}
        if(idx<text.length)frag.appendChild(document.createTextNode(text.slice(idx)));
        node.parentNode.replaceChild(frag,node)});
      if(findn)findn.textContent=marks.length?('0/'+marks.length):'0';
      if(marks.length){cur=0;focusMark()}}
    var find=document.getElementById('find');
    function updClear(){if(findclear)findclear.style.display=find&&find.value?'inline-flex':'none'}
    if(find){find.addEventListener('input',function(){doFind(find.value.trim());updClear()});
      find.addEventListener('keydown',function(e){if(e.key==='Enter'){e.preventDefault();step(e.shiftKey?-1:1)}else if(e.key==='Escape'){find.value='';clearFind();updClear()}})}
    if(findclear)findclear.addEventListener('click',function(){if(find){find.value='';find.focus()}clearFind();updClear()});
    updClear();
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
