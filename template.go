package main

// pageHTML is the single-page reader shell, ported from the standalone Python
// md_reader.py and adapted for mdserve. It renders Markdown client-side with the
// embedded vendor bundle (marked + highlight.js + mermaid + KaTeX, all under
// /vendor/), so the page is fully offline. Differences from the original:
//   - all icons are Google Material Symbols (Rounded) instead of hand-drawn ones
//   - the bookmark / "where you left off" reading marker is removed
//   - the sidebar carries an mdserve logo + title above the file filter
//   - per-doc scroll position lives in localStorage (no server-side state API)
//
// The two sentinels __RELOAD__ and __DEFAULT__ are substituted per server in
// NewServer (a JS boolean and the default-doc rel path). The doc tree, file
// contents, and change polling come from /api/tree, /raw and /api/poll.
const pageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>mdserve</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='7' fill='%232f81f7'/%3E%3Cpath d='M9 11h14M9 16h14M9 21h9' fill='none' stroke='white' stroke-width='2.4' stroke-linecap='round'/%3E%3C/svg%3E">
<link id="hljs-theme" rel="stylesheet" href="/vendor/hljs-dark.css">
<link rel="stylesheet" href="/vendor/katex.min.css">
<script>window.MDSERVE={reload:__RELOAD__,defaultDoc:"__DEFAULT__"};</script>
<style>
:root{
  --speed:.18s;
  --bar:50px;          /* shared top-bar height: toolbar + rail headers */
}
[data-theme="warm"]{
  --app-bg:#edece6; --rail-bg:#f5f4ef; --paper:#fbfaf6; --ink:#34352f;
  --muted:#8c8b80; --faint:#b6b4a6; --accent:#8a7f6a; --accent-soft:#cdc8ba;
  --active:#e8e5db; --border:#e5e2d8; --code-bg:#f1efe8; --code-ink:#56534a; --sel:#e9e4d6; --sel-strong:#b5651d; --logo:#b5651d;
  --shadow:0 1px 2px rgba(60,55,40,.05),0 8px 26px rgba(60,55,40,.06); --hljs-filter:none;
}
[data-theme="light"]{
  --app-bg:#eceef1; --rail-bg:#f6f7f9; --paper:#ffffff; --ink:#23272e;
  --muted:#6b7480; --faint:#aab1bb; --accent:#5b6570; --accent-soft:#cdd3da;
  --active:#e7eaef; --border:#e6e9ee; --code-bg:#f4f6f8; --code-ink:#2f363d; --sel:#d7dde6; --sel-strong:#0969da; --logo:#0969da;
  --shadow:0 1px 2px rgba(40,50,70,.05),0 8px 26px rgba(40,50,70,.07); --hljs-filter:none;
}
[data-theme="dark"]{
  --app-bg:#010409; --rail-bg:#0d1117; --paper:#0d1117; --ink:#e6edf3;
  --muted:#8b949e; --faint:#6e7681; --accent:#2f81f7; --accent-soft:#2b466b;
  --active:#21262d; --border:#30363d; --code-bg:#161b22; --code-ink:#c9d1d9; --sel:#26354d; --sel-strong:#2f81f7; --logo:#2f81f7;
  --shadow:0 1px 2px rgba(0,0,0,.3),0 10px 30px rgba(0,0,0,.5); --hljs-filter:invert(.92) hue-rotate(180deg);
}
*{box-sizing:border-box}
html,body{height:100%;margin:0}
body{
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  background:var(--app-bg); color:var(--ink);
  display:flex; overflow:hidden; transition:background var(--speed);
}
::selection{background:var(--sel)}
svg{fill:currentColor}
/* reading font, toggled by the toolbar (serif default, sans optional) */
body[data-font="serif"]{--read-font:"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Times New Roman",serif}
body[data-font="sans"]{--read-font:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}

/* ---- left sidebar : brand + filter + file tree ---- */
#sidebar{
  flex:none; width:264px; background:var(--rail-bg); border-right:1px solid var(--border);
  display:flex; flex-direction:column; overflow:hidden; position:relative;
  transition:width var(--speed),margin var(--speed);
}
#sidebar.collapsed{width:0!important;margin-left:-1px;border:none}
.collapsed .rail-resize{display:none}
.rail-resize{position:absolute;top:0;bottom:0;width:6px;cursor:col-resize;z-index:7}
.rail-resize:hover,.rail-resize.drag{background:var(--accent-soft)}
#sidebar .rail-resize{right:0}
#outline .rail-resize{left:0}
/* mdserve brand, sits ABOVE the file filter */
#brand{
  height:var(--bar); flex:none; display:flex; align-items:center; gap:9px;
  padding:0 13px; border-bottom:1px solid var(--border);
  font-weight:700; color:var(--ink); white-space:nowrap; overflow:hidden; user-select:none;
}
#brand .logo{width:22px;height:22px;border-radius:6px;flex:none}
#brand .logo rect{fill:var(--logo)}
#brand .name{font-size:15px;letter-spacing:.01em}
/* version footer pinned to the bottom of the sidebar (takes a sliver off the tree) */
#sidefoot{flex:none;padding:7px 13px;border-top:1px solid var(--border);
  font-size:11px;color:var(--faint);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;
  font-variant-numeric:tabular-nums;letter-spacing:.01em}
#sidefoot .v{color:var(--muted);font-weight:600}
#brand .cwd{font-size:12px;font-weight:500;color:var(--muted);overflow:hidden;text-overflow:ellipsis;
  white-space:nowrap;max-width:120px;padding-left:9px;margin-left:2px;border-left:1px solid var(--border)}
#brand .cwd:empty{display:none}
.rail-top{
  height:var(--bar); flex:none; display:flex; align-items:center; gap:8px;
  padding:0 10px; border-bottom:1px solid var(--border); white-space:nowrap;
}
.rail-title{
  font-size:12px; letter-spacing:.13em; text-transform:uppercase;
  color:var(--muted); font-weight:700;
}
.filter{flex:1;margin:0;position:relative}
.filter input{width:100%;border:1px solid var(--border);background:var(--paper);color:var(--ink);
  border-radius:8px;padding:7px 26px 7px 30px;font-size:13px;outline:none;transition:border-color .12s}
.filter input::placeholder{color:var(--faint)}
.filter input:focus{border-color:var(--accent-soft)}
.filter .sicon{position:absolute;left:9px;top:50%;transform:translateY(-50%);width:14px;height:14px;
  color:var(--faint);pointer-events:none;display:flex}
.filter .sicon svg{width:14px;height:14px}
.filter .clr{position:absolute;right:7px;top:50%;transform:translateY(-50%);width:16px;height:16px;
  border:none;background:none;color:var(--faint);cursor:pointer;display:none;align-items:center;justify-content:center;padding:0}
.filter.has .clr{display:flex}
.filter .clr svg{width:14px;height:14px}
#tree{overflow:auto; padding:4px 8px 18px; flex:1}
#tree .empty-filter{padding:10px 12px;color:var(--faint);font-size:12px}
.node{user-select:none}
.node.hide{display:none}
.row{
  display:flex; align-items:center; gap:7px; padding:5px 8px; border-radius:8px;
  cursor:pointer; font-size:13.5px; color:var(--ink); white-space:nowrap; line-height:1.2;
}
.row:hover{background:rgba(0,0,0,.06)}
[data-theme="dark"] .row:hover{background:rgba(255,255,255,.06)}
.row.active,.row.active:hover{background:var(--sel-strong); color:#fff; font-weight:600}
.row.active .ico{opacity:1;color:#fff}
.row .ico{width:16px;height:16px;opacity:.75;flex:none;color:var(--muted);display:flex;align-items:center;justify-content:center}
.row .ico svg{width:16px;height:16px}
.row .ico .f-open,.row .ico .f-closed{display:flex;align-items:center;justify-content:center}
.row .ico .f-closed{display:none}
.node.closed > .row .ico .f-open{display:none}
.node.closed > .row .ico .f-closed{display:flex}
.row .caret{width:14px;height:14px;transition:transform var(--speed);opacity:.55;flex:none;display:flex;align-items:center;justify-content:center}
.row .caret svg{width:14px;height:14px}
.node.closed > .children{display:none}
.node.closed > .row .caret{transform:rotate(-90deg)}
.children{margin-left:13px;border-left:1px dashed var(--border);padding-left:5px}
.row .name{overflow:hidden;text-overflow:ellipsis}

/* ---- center : toolbar + viewport ---- */
#main{flex:1;display:flex;flex-direction:column;overflow:hidden;min-width:0;position:relative}
#toolbar{
  height:var(--bar); flex:none; display:flex; align-items:center; gap:6px;
  padding:0 12px; background:var(--rail-bg); border-bottom:1px solid var(--border); z-index:5;
}
#progress{position:absolute;top:var(--bar);left:0;height:2px;width:0;background:var(--accent);
  opacity:.55;z-index:6;pointer-events:none;transition:width .05s linear}
.tbtn{
  display:inline-flex;align-items:center;justify-content:center;gap:6px;
  height:32px;min-width:32px;padding:0 8px;border:1px solid transparent;
  background:transparent;color:var(--muted);border-radius:8px;cursor:pointer;
  font-size:13px;line-height:1;transition:all .12s;
}
.tbtn svg{width:19px;height:19px;display:block}
.tbtn:hover{background:var(--active);color:var(--ink)}
.tbtn.on{background:var(--active);color:var(--accent);border-color:var(--border)}
.tbtn:active{transform:translateY(1px)}
.sep{width:1px;height:20px;background:var(--border);margin:0 3px}
#zoomlbl{font-size:13px;min-width:46px;text-align:center;color:var(--muted);font-variant-numeric:tabular-nums}
#crumb{
  margin-left:auto;font-size:13px;color:var(--muted);white-space:nowrap;
  overflow:hidden;text-overflow:ellipsis;max-width:40vw;padding-left:10px;
}
#crumb b{color:var(--ink);font-weight:600}

#viewport{flex:1; overflow:auto; position:relative; background:var(--paper)}
#viewport.hand{cursor:grab; user-select:none}
#viewport.hand.grabbing{cursor:grabbing}
#viewport.hand #page{pointer-events:none}
#canvas{ width:780px; margin:0 auto; }
#page{
  width:780px; position:relative; background:transparent; color:var(--ink);
  padding:40px 48px 96px;
  font-family:var(--read-font,"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Times New Roman",serif);
  font-size:16px; line-height:1.68; transform-origin:0 0;
  overflow-wrap:break-word;
}
#empty{max-width:620px;margin:14vh auto;text-align:center;color:var(--muted);font-family:-apple-system,sans-serif}
#empty h2{font-size:20px;color:var(--ink);margin:0 0 8px}
#empty code{background:var(--code-bg);padding:2px 7px;border-radius:6px}

/* ---- markdown content styling ---- */
#page h1,#page h2,#page h3,#page h4{line-height:1.25;font-weight:700;
  scroll-margin-top:24px;font-family:-apple-system,"Segoe UI",sans-serif}
#page h1{font-size:2.05em;margin:.2em 0 .55em;border-bottom:2px solid var(--border);padding-bottom:.28em}
#page h2{font-size:1.5em;margin:1.7em 0 .6em;border-bottom:1px solid var(--border);padding-bottom:.22em}
#page h3{font-size:1.22em;margin:1.5em 0 .5em}
#page h4{font-size:1.05em;margin:1.3em 0 .4em;color:var(--muted)}
#page p{margin:0 0 1.05em}
#page a{color:var(--accent);text-decoration:none;border-bottom:1px solid var(--accent-soft)}
#page a:hover{background:var(--sel)}
#page ul,#page ol{margin:0 0 1.05em;padding-left:1.5em}
#page li{margin:.25em 0}
#page li::marker{color:var(--accent-soft)}
#page input[type=checkbox]{margin-right:.5em;transform:scale(1.15);accent-color:var(--accent)}
#page ul.contains-task-list,#page .task-list-item{list-style:none}
#page .task-list-item{margin-left:-1.2em}
#page blockquote{margin:1.2em 0;padding:.5em 1.1em;border-left:4px solid var(--accent-soft);
  background:rgba(0,0,0,.03);color:var(--muted);border-radius:0 8px 8px 0}
[data-theme="dark"] #page blockquote{background:rgba(255,255,255,.04)}
#page hr{border:none;border-top:1px solid var(--border);margin:2em 0}
#page img{max-width:100%;border-radius:8px;display:block;margin:1em auto}
#page code{font-family:"SF Mono",ui-monospace,Menlo,Consolas,monospace;font-size:.86em;
  background:var(--code-bg);color:var(--code-ink);padding:.15em .42em;border-radius:5px}
#page pre{background:var(--code-bg);border:1px solid var(--border);border-radius:11px;
  padding:16px 18px;overflow:auto;margin:0 0 1.15em;line-height:1.55}
#page pre code{background:none;padding:0;font-size:13px}
#page .table-wrap{max-width:100%;overflow-x:auto;margin:0 0 1.2em}
#page table{border-collapse:collapse;width:auto;margin:0;font-size:.92em;font-family:-apple-system,sans-serif}
#page th,#page td{border:1px solid var(--border);padding:8px 13px;text-align:left}
#page th{background:var(--code-bg);font-weight:700}
#page tr:nth-child(even) td{background:rgba(0,0,0,.02)}
[data-theme="dark"] #page tr:nth-child(even) td{background:rgba(255,255,255,.03)}
#page .mermaid{margin:1.3em 0;text-align:center;background:transparent}
#page .mermaid svg{max-width:100%;height:auto}
#page .katex-display{overflow:auto hidden;padding:.3em 0}
#page del{color:var(--faint)}
#page h1:first-child,#page h2:first-child{margin-top:.1em}

/* ---- right rail : outline ---- */
#outline{
  flex:none; width:248px;background:var(--rail-bg);border-left:1px solid var(--border);
  display:flex;flex-direction:column;overflow:hidden;position:relative;
  transition:width var(--speed),margin var(--speed);
}
#outline.collapsed{width:0!important;margin-right:-1px;border:none}
#toc{overflow:auto;padding:4px 10px 20px;flex:1}
#toc a{display:block;padding:5px 10px;border-radius:7px;font-size:13px;color:var(--muted);text-decoration:none;
  white-space:nowrap;overflow:hidden;text-overflow:ellipsis;line-height:1.35}
#toc a:hover{color:var(--ink);background:rgba(0,0,0,.05)}
[data-theme="dark"] #toc a:hover{background:rgba(255,255,255,.05)}
#toc a.lvl2{padding-left:22px;font-size:12.5px}
#toc a.lvl3{padding-left:34px;font-size:12px}
#toc a.active{color:var(--sel-strong);font-weight:700;background:rgba(0,0,0,.04)}
[data-theme="dark"] #toc a.active{background:rgba(255,255,255,.05)}

/* scrollbars */
::-webkit-scrollbar{width:11px;height:11px}
::-webkit-scrollbar-thumb{background:var(--faint);border-radius:8px;border:3px solid var(--rail-bg)}
#viewport::-webkit-scrollbar-thumb{border-color:var(--app-bg)}
::-webkit-scrollbar-track{background:transparent}

/* floating "go to top" button */
#fab-up{position:absolute;right:22px;bottom:22px;width:40px;height:40px;border-radius:50%;
  background:var(--paper);color:var(--accent);border:1px solid var(--border);box-shadow:var(--shadow);
  cursor:pointer;display:flex;align-items:center;justify-content:center;z-index:8;
  opacity:0;pointer-events:none;transform:translateY(8px);transition:opacity .18s,transform .18s}
#fab-up.show{opacity:1;pointer-events:auto;transform:none}
#fab-up:hover{border-color:var(--accent-soft);color:var(--ink)}
#fab-up svg{width:22px;height:22px}

/* in-page find bar */
#findbar{position:absolute;top:calc(var(--bar) + 8px);right:18px;z-index:9;display:none;
  align-items:center;gap:2px;background:var(--rail-bg);border:1px solid var(--border);
  border-radius:10px;padding:5px 6px;box-shadow:var(--shadow)}
#findbar.show{display:flex}
#find-in{border:none;background:transparent;color:var(--ink);outline:none;font-size:13px;width:170px;padding:4px 8px}
#find-in::placeholder{color:var(--faint)}
#find-count{font-size:12px;color:var(--muted);min-width:46px;text-align:center;font-variant-numeric:tabular-nums}
mark.find{background:#ffe08a;color:#000;border-radius:2px;padding:0 1px}
[data-theme="dark"] mark.find{background:#6e5713;color:#fff}
mark.find.cur{background:#ff9f43;color:#000;box-shadow:0 0 0 2px #e07b1e}

@media print{
  /* chrome and overlays off */
  #sidebar,#outline,#toolbar,#progress,#findbar,#fab-up{display:none!important}
  html,body{height:auto!important;overflow:visible!important}
  body{display:block;background:#fff}
  #main{overflow:visible!important}
  #viewport{position:static!important;overflow:visible!important}
  #canvas{width:auto!important;height:auto!important;margin:0!important}
  /* the live page carries an inline pixel width + transform from zoom/layout;
     override both so content reflows to the paper instead of running off it */
  #page{box-shadow:none!important;border:none!important;transform:none!important;
    width:auto!important;max-width:100%!important;padding:0!important}
  /* wrap long code / cells that otherwise overflow the page width */
  #page pre,#page pre code{white-space:pre-wrap!important;word-break:break-word!important;overflow:visible!important}
  #page code{word-break:break-word!important}
  #page .table-wrap{overflow:visible!important}
  #page th,#page td{word-break:break-word!important}
  #page img{max-width:100%!important}
}
</style>
</head>
<body data-theme="dark" data-font="sans">

<aside id="sidebar">
  <div id="brand"><svg class="logo" viewBox="0 0 32 32"><rect width="32" height="32" rx="7" fill="#2f81f7"/><path d="M9 11h14M9 16h14M9 21h9" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/></svg><span class="name">mdserve</span><span class="cwd" id="rootname"></span></div>
  <div class="rail-top">
    <div class="filter">
      <span class="sicon"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M440-160q-17 0-28.5-11.5T400-200v-240L161-745q-14-17-4-36t31-19h584q21 0 31 19t-4 36L560-440v240q0 17-11.5 28.5T520-160h-80Z"/></svg></span>
      <input id="filter" type="text" placeholder="Filter docs  ( / )" spellcheck="false" autocomplete="off">
      <button class="clr" id="b-clr" title="Clear"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-438 270-228q-9 9-21 9t-21-9q-9-9-9-21t9-21l210-210-210-210q-9-9-9-21t9-21q9-9 21-9t21 9l210 210 210-210q9-9 21-9t21 9q9 9 9 21t-9 21L522-480l210 210q9 9 9 21t-9 21q-9 9-21 9t-21-9L480-438Z"/></svg></button>
    </div>
  </div>
  <div id="tree"></div>
  <div id="sidefoot">mdserve <span class="v">__VERSION__</span></div>
  <div class="rail-resize" title="Drag to resize"></div>
</aside>

<section id="main">
  <div id="toolbar">
    <button class="tbtn" id="b-side" title="Toggle file list (Cmd/Ctrl B)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M663-380v-200q0-9.92-9.5-13.46Q644-597 637-590l-89 89q-9 9-9 21t9 21l89 89q7 7 16.5 3.46T663-380ZM180-120q-24.75 0-42.37-17.63Q120-155.25 120-180v-600q0-24.75 17.63-42.38Q155.25-840 180-840h600q24.75 0 42.38 17.62Q840-804.75 840-780v600q0 24.75-17.62 42.37Q804.75-120 780-120H180Zm207-60h393v-600H387v600Z"/></svg></button>
    <div class="sep"></div>
    <button class="tbtn" id="b-zout" title="Zoom out (Ctrl -)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M305-556q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h141q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H305Zm73 227q-108.16 0-183.08-75Q120-479 120-585t75-181q75-75 181.5-75t181 75Q632-691 632-584.85 632-542 618-502q-14 40-42 75l242 240q9 8.56 9 21.78T818-143q-9 9-22.22 9-13.22 0-21.78-9L533-384q-30 26-69.96 40.5Q423.08-329 378-329Zm-1-60q81.25 0 138.13-57.5Q572-504 572-585t-56.87-138.5Q458.25-781 377-781q-82.08 0-139.54 57.5Q180-666 180-585t57.46 138.5Q294.92-389 377-389Z"/></svg></button>
    <span id="zoomlbl">100%</span>
    <button class="tbtn" id="b-zin" title="Zoom in (Ctrl +)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M346-556h-52q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h52v-51q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v51h51q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5h-51v52q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-52Zm32 227q-108.16 0-183.08-75Q120-479 120-585t75-181q75-75 181.5-75t181 75Q632-691 632-584.85 632-542 618-502q-14 40-42 75l242 240q9 8.56 9 21.78T818-143q-9 9-22.22 9-13.22 0-21.78-9L533-384q-30 26-69.96 40.5Q423.08-329 378-329Zm-1-60q81.25 0 138.13-57.5Q572-504 572-585t-56.87-138.5Q458.25-781 377-781q-82.08 0-139.54 57.5Q180-666 180-585t57.46 138.5Q294.92-389 377-389Z"/></svg></button>
    <button class="tbtn" id="b-zreset" title="Reset zoom to 100% (Ctrl 0)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M481-158q-131 0-225.5-94.5T161-478v-45l-61 61q-8 8-19 8t-19-8q-8-8-8-19.5t8-19.5l108-109q9-9 21-9t21 9l109 109q8 8 8 19t-8 19q-8 8-19.5 8t-19.5-8l-61-60v45q0 107 76.5 183.5T481-218q20 0 39-2.5t36-7.5q12-3 23.5 1.5T596-211q5 11 0 22t-17 15q-24 8-48.5 12t-49.5 4Zm-1-580q-20 0-39 2.5t-36 7.5q-12 3-24-1.5T364-745q-5-11 0-22.5t16-15.5q25-8 49.5-11.5T480-798q131 0 225.5 94.5T800-478v43l61-61q8-8 19-8t19 8q8 8 8 19.5t-8 19.5L791-348q-9 9-21 9t-21-9L641-456q-8-8-8-20t8-20q8-8 20-8t20 8l59 59v-41q0-107-76.5-183.5T480-738Z"/></svg></button>
    <div class="sep"></div>
    <button class="tbtn on" id="b-select" title="Select text"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M605-105q-19 9-38 2t-28-26L412-401 294-236q-13 18-33.5 11T240-254v-564q0-19 17-27t32 3l443 348q17 14 9.5 34T713-440H505l124 269q9 19 2 38t-26 28Z"/></svg></button>
    <button class="tbtn" id="b-hand" title="Hand tool — drag to pan (H)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M402-40q-27 0-51.5-12.5T311-88L68-446q-6-9-4-20t10-18q17-15 39.5-19t44.57 13.19L280-397v-413q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v330h107v-410q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v410h107v-370q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v370h106v-290q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v580q0 63-43.5 106.5T690-40H402Z"/></svg></button>
    <div class="sep"></div>
    <button class="tbtn" id="b-theme" title="Cycle theme (warm / light / dark)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-120q-150 0-255-105T120-480q0-135 79.5-229T408-830q20-5 34-1t22 15q8 10 7.5 25t-8.5 35q-9 23-14 47t-5 49q0 90 63 153t153 63q25 0 48.5-4.5T754-461q22-8 38-7t26 9q10 8 13 23t-2 36q-27 121-121 200.5T480-120Z"/></svg></button>
    <button class="tbtn" id="b-font" title="Reading font (serif / sans)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M304.5-174.58Q290-189.17 290-210v-490H130q-20.83 0-35.42-14.62Q80-729.24 80-750.12 80-771 94.58-785.5 109.17-800 130-800h420q20.83 0 35.42 14.62Q600-770.76 600-749.88q0 20.88-14.58 35.38Q570.83-700 550-700H390v490q0 20.83-14.62 35.42Q360.76-160 339.88-160q-20.88 0-35.38-14.58Zm360 0Q650-189.17 650-210v-290h-80q-20.83 0-35.42-14.62Q520-529.24 520-550.12q0-20.88 14.58-35.38Q549.17-600 570-600h260q20.83 0 35.42 14.62Q880-570.76 880-549.88q0 20.88-14.58 35.38Q850.83-500 830-500h-80v290q0 20.83-14.62 35.42Q720.76-160 699.88-160q-20.88 0-35.38-14.58Z"/></svg></button>
    <button class="tbtn" id="b-full" title="Full screen"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M180-180h103q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H150q-12.75 0-21.37-8.63Q120-137.25 120-150v-133q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v103Zm600 0v-103q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v133q0 12.75-8.62 21.37Q822.75-120 810-120H677q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h103ZM180-780v103q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-133q0-12.75 8.63-21.38Q137.25-840 150-840h133q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H180Zm600 0H677q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h133q12.75 0 21.38 8.62Q840-822.75 840-810v133q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-103Z"/></svg></button>
    <button class="tbtn" id="b-print" title="Print / save as PDF"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M302-120q-24.75 0-42.37-17.63Q242-155.25 242-180v-116H140q-24.75 0-42.37-17.63Q80-331.25 80-356v-186q0-45.05 30.5-75.53Q141-648 186-648h588q45.05 0 75.53 30.47Q880-587.05 880-542v186q0 24.75-17.62 42.37Q844.75-296 820-296H718v116q0 24.75-17.62 42.37Q682.75-120 658-120H302Zm416-558H242v-102q0-24.75 17.63-42.38Q277.25-840 302-840h356q24.75 0 42.38 17.62Q718-804.75 718-780v102Zm21 185q12 0 21-9t9-21q0-12-9-21t-21-9q-12 0-21 9t-9 21q0 12 9 21t21 9ZM302-180h356v-192H302v192Z"/></svg></button>
    <button class="tbtn" id="b-find" title="Find in page (Cmd/Ctrl F)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M378-329q-108.16 0-183.08-75Q120-479 120-585t75-181q75-75 181.5-75t181 75Q632-691 632-584.85 632-542 618-502q-14 40-42 75l242 240q9 8.56 9 21.78T818-143q-9 9-22.22 9-13.22 0-21.78-9L533-384q-30 26-69.96 40.5Q423.08-329 378-329Zm-1-60q81.25 0 138.13-57.5Q572-504 572-585t-56.87-138.5Q458.25-781 377-781q-82.08 0-139.54 57.5Q180-666 180-585t57.46 138.5Q294.92-389 377-389Z"/></svg></button>
    <div id="crumb"></div>
    <button class="tbtn" id="b-out" title="Toggle outline (Cmd/Ctrl \\)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M150-280q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h490q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H150Zm0-170q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h490q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H150Zm0-170q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h490q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H150Zm659.5 340q-12.5 0-21-8.63-8.5-8.62-8.5-21.37 0-12 8.63-21 8.62-9 21.37-9 12 0 21 9t9 21.5q0 12.5-9 21t-21.5 8.5Zm0-170q-12.5 0-21-8.63-8.5-8.62-8.5-21.37 0-12 8.63-21 8.62-9 21.37-9 12 0 21 9t9 21.5q0 12.5-9 21t-21.5 8.5Zm0-170q-12.5 0-21-8.63-8.5-8.62-8.5-21.37 0-12 8.63-21 8.62-9 21.37-9 12 0 21 9t9 21.5q0 12.5-9 21t-21.5 8.5Z"/></svg></button>
  </div>
  <div id="progress"></div>
  <div id="findbar">
    <input id="find-in" type="text" placeholder="Find in page" spellcheck="false" autocomplete="off">
    <span id="find-count">0/0</span>
    <button class="tbtn" id="find-prev" title="Previous (Shift+Enter)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-554 304-378q-9 9-21 8.5t-21-9.5q-9-9-9-21.5t9-21.5l197-197q9-9 21-9t21 9l198 198q9 9 9 21t-9 21q-9 9-21.5 9t-21.5-9L480-554Z"/></svg></button>
    <button class="tbtn" id="find-next" title="Next (Enter)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M469-358q-5-2-10-7L261-563q-9-9-8.5-21.5T262-606q9-9 21.5-9t21.5 9l175 176 176-176q9-9 21-8.5t21 9.5q9 9 9 21.5t-9 21.5L501-365q-5 5-10 7t-11 2q-6 0-11-2Z"/></svg></button>
    <button class="tbtn" id="find-close" title="Close (Esc)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-438 270-228q-9 9-21 9t-21-9q-9-9-9-21t9-21l210-210-210-210q-9-9-9-21t9-21q9-9 21-9t21 9l210 210 210-210q9-9 21-9t21 9q9 9 9 21t-9 21L522-480l210 210q9 9 9 21t-9 21q-9 9-21 9t-21-9L480-438Z"/></svg></button>
  </div>
  <div id="viewport">
    <div id="canvas"><div id="page"></div></div>
  </div>
  <button id="fab-up" title="Go to top"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M450-686 223-459q-9 9-21 9t-21-9q-9-9-9-21t9-21l278-278q5-5 10-7t11-2q6 0 11 2t10 7l278 278q9 9 9 21t-9 21q-9 9-21 9t-21-9L510-686v496q0 13-8.5 21.5T480-160q-13 0-21.5-8.5T450-190v-496Z"/></svg></button>
</section>

<aside id="outline">
  <div class="rail-resize" title="Drag to resize"></div>
  <div class="rail-top"><span class="rail-title">Navigation</span></div>
  <div id="toc"></div>
</aside>

<script src="/vendor/marked.min.js"></script>
<script src="/vendor/highlight.min.js"></script>
<script src="/vendor/mermaid.min.js"></script>
<script src="/vendor/katex.min.js"></script>
<script src="/vendor/auto-render.min.js"></script>
<script>
"use strict";
const $ = s => document.querySelector(s);
const page = $("#page"), viewport = $("#viewport"), toc = $("#toc"), canvas = $("#canvas");
let state = { path:null, zoom:1, tool:"select", theme:"dark", mtimes:{}, treeKey:"", obs:null };
let bootTarget = "";
const THEMES = ["warm","light","dark"];
const mermaidTheme = t => t==="dark" ? "dark" : (t==="warm" ? "neutral" : "default");
// Google Material Symbols (Rounded), wrapped as inline svgs that fill currentColor.
const MS = p => '<svg viewBox="0 -960 960 960" fill="currentColor"><path d="'+p+'"/></svg>';
const PATH = {
  expandMore: 'M469-358q-5-2-10-7L261-563q-9-9-8.5-21.5T262-606q9-9 21.5-9t21.5 9l175 176 176-176q9-9 21-8.5t21 9.5q9 9 9 21.5t-9 21.5L501-365q-5 5-10 7t-11 2q-6 0-11-2Z',
  folder: 'M140-160q-24 0-42-18.5T80-220v-520q0-23 18-41.5t42-18.5h256q12 0 23.5 5t19.5 13l42 42h339q23 0 41.5 18.5T880-680v460q0 23-18.5 41.5T820-160H140Z',
  folderOpen: 'M140-160q-23 0-41.5-18.5T80-220v-520q0-23 18.5-41.5T140-800h256q12 0 23.5 5t19.5 13l42 42h369q13 0 21.5 8.5T880-710q0 13-8.5 21.5T850-680H289q-57 0-103 28t-46 79v353l90-355q5-20 22-32.5t37-12.5h574q29 0 47.5 23t10.5 52l-88 339q-6 24-22 35t-41 11H140Z',
  file: 'M349-250h262q13 0 21.5-8.5T641-280q0-13-8.5-21.5T611-310H349q-13 0-21.5 8.5T319-280q0 13 8.5 21.5T349-250Zm0-170h262q13 0 21.5-8.5T641-450q0-13-8.5-21.5T611-480H349q-13 0-21.5 8.5T319-450q0 13 8.5 21.5T349-420ZM220-80q-24 0-42-18t-18-42v-680q0-24 18-42t42-18h336q12 0 23.5 5t19.5 13l183 183q8 8 13 19.5t5 23.5v496q0 24-18 42t-42 18H220Zm331-584q0 13 8.5 21.5T581-634h159L551-820v156Z',
  panelClose: 'M663-380v-200q0-9.92-9.5-13.46Q644-597 637-590l-89 89q-9 9-9 21t9 21l89 89q7 7 16.5 3.46T663-380ZM180-120q-24.75 0-42.37-17.63Q120-155.25 120-180v-600q0-24.75 17.63-42.38Q155.25-840 180-840h600q24.75 0 42.38 17.62Q840-804.75 840-780v600q0 24.75-17.62 42.37Q804.75-120 780-120H180Zm207-60h393v-600H387v600Z',
  panelOpen: 'M527-580v200q0 9.92 9.5 13.46Q546-363 553-370l89-89q9-9 9-21t-9-21l-89-89q-7-7-16.5-3.46T527-580ZM180-120q-24.75 0-42.37-17.63Q120-155.25 120-180v-600q0-24.75 17.63-42.38Q155.25-840 180-840h600q24.75 0 42.38 17.62Q840-804.75 840-780v600q0 24.75-17.62 42.37Q804.75-120 780-120H180Zm207-60h393v-600H387v600Z',
  full: 'M180-180h103q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H150q-12.75 0-21.37-8.63Q120-137.25 120-150v-133q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v103Zm600 0v-103q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v133q0 12.75-8.62 21.37Q822.75-120 810-120H677q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h103ZM180-780v103q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-133q0-12.75 8.63-21.38Q137.25-840 150-840h133q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H180Zm600 0H677q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h133q12.75 0 21.38 8.62Q840-822.75 840-810v133q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-103Z',
  fullExit: 'M253-253H150q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h133q12.75 0 21.38 8.62Q313-295.75 313-283v133q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-103Zm454 0v103q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-133q0-12.75 8.63-21.38Q664.25-313 677-313h133q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H707ZM253-707v-103q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v133q0 12.75-8.62 21.37Q295.75-647 283-647H150q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h103Zm454 0h103q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H677q-12.75 0-21.37-8.63Q647-664.25 647-677v-133q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v103Z',
  warm: 'M792-670q9 9 9 21t-9 21l-43 43q-9.07 9-21.53 9-12.47 0-21.34-9.05-9.13-9.06-8.63-21.5Q698-619 707-628l43-42q9-8 21-8.5t21 8.5ZM110-170q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32Q97.25-230 110-230h740q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H110Zm391.5-621.38q8.5 8.63 8.5 21.38v60q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63-8.5-8.62-8.5-21.37v-60q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62ZM169-672q9-9 21-9t21 9l44 44q9 9.07 9 21.53 0 12.47-9 21.86-9 8.61-21.5 8.11T212-586l-43-44q-8-9-8.5-21t8.5-21Zm31 352q0-117 81.5-198.5T480-600q117 0 198.5 81.5T760-320H200Z',
  light: 'M338.5-338.5Q280-397 280-480t58.5-141.5Q397-680 480-680t141.5 58.5Q680-563 680-480t-58.5 141.5Q563-280 480-280t-141.5-58.5ZM70-450q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32Q57.25-510 70-510h100q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H70Zm720 0q-12.75 0-21.37-8.68-8.63-8.67-8.63-21.5 0-12.82 8.63-21.32 8.62-8.5 21.37-8.5h100q12.75 0 21.38 8.68 8.62 8.67 8.62 21.5 0 12.82-8.62 21.32-8.63 8.5-21.38 8.5H790ZM458.5-768.63Q450-777.25 450-790v-100q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v100q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63Zm0 720Q450-57.25 450-70v-100q0-12.75 8.68-21.38 8.67-8.62 21.5-8.62 12.82 0 21.32 8.62 8.5 8.63 8.5 21.38v100q0 12.75-8.68 21.37-8.67 8.63-21.5 8.63-12.82 0-21.32-8.63ZM240-678l-57-56q-9-9-8.63-21.6.37-12.61 8.53-21.5 8.89-8.9 21.5-8.9 12.6 0 21.6 9l56 57q8 9 8 21t-8 20.5q-8 8.5-20.5 8.5t-21.5-8Zm494 495-56-57q-8-9-8-21.38 0-12.37 8.5-20.62 8.5-9 20.5-9t21 9l57 56q9 9 8.63 21.6-.37 12.61-8.53 21.5-8.89 8.9-21.5 8.9-12.6 0-21.6-9Zm-56-495q-9-9-9-21t9-21l56-57q9-9 21.6-8.63 12.61.37 21.5 8.53 8.9 8.89 8.9 21.5 0 12.6-9 21.6l-57 56q-8 8-20.36 8-12.37 0-21.64-8ZM182.9-182.9q-8.9-8.89-8.9-21.5 0-12.6 9-21.6l57-56q8.8-9 20.9-9 12.1 0 20.71 9 9.39 9 9.39 21t-9 21l-56 57q-9 9-21.6 8.63-12.61-.37-21.5-8.53Z',
  dark: 'M480-120q-150 0-255-105T120-480q0-135 79.5-229T408-830q20-5 34-1t22 15q8 10 7.5 25t-8.5 35q-9 23-14 47t-5 49q0 90 63 153t153 63q25 0 48.5-4.5T754-461q22-8 38-7t26 9q10 8 13 23t-2 36q-27 121-121 200.5T480-120Z'
};
const ICONS = {
  chevron: MS(PATH.expandMore),
  folder:  MS(PATH.folder),
  folderOpen: MS(PATH.folderOpen),
  file:    MS(PATH.file),
  theme: { warm: MS(PATH.warm), light: MS(PATH.light), dark: MS(PATH.dark) }
};

// Apply fn to every text node under root, skipping code/math/diagram subtrees.
// Used to shield currency ($100) from KaTeX's $...$ inline-math delimiter.
function mathWalk(root, fn){
  const w=document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {acceptNode(n){
    return n.parentElement && n.parentElement.closest("pre,code,script,style,textarea,.katex,.mermaid")
      ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT; }});
  const out=[]; for(let n; n=w.nextNode();) out.push(n);
  for(const n of out){ const v=fn(n.nodeValue); if(v!==n.nodeValue) n.nodeValue=v; }
}

marked.setOptions({ gfm:true, breaks:false, headerIds:false, mangle:false });

/* ---------- file tree ---------- */
async function loadTree(){
  const data = await (await fetch("/api/tree")).json();
  $("#rootname").textContent = data.root || "";
  $("#rootname").title = data.rootPath || data.root || "";
  const key = JSON.stringify(stripMtime(data.tree));
  if(key === state.treeKey) return data;
  state.treeKey = key;
  $("#tree").innerHTML = "";
  $("#tree").appendChild(renderNodes(data.tree));
  highlightActive();
  if(!state.path){
    const want = bootTarget && fileInTree(bootTarget, data.tree) ? bootTarget : firstFile(data.tree);
    if(want) openFile(want);
    else showEmpty();
  }
  return data;
}
function fileInTree(rel, nodes){
  for(const n of nodes){
    if(n.type==="file" && n.relpath===rel) return true;
    if(n.type==="dir" && fileInTree(rel, n.children||[])) return true;
  }
  return false;
}
function stripMtime(nodes){
  return nodes.map(n => n.type==="dir"
    ? {n:n.name, c:stripMtime(n.children)} : {f:n.relpath});
}
// firstFile picks the doc to open on load: prefer a file at the current level
// (any "readme*" first, then index/home, else the first file here); only if this
// level has no files does it descend into folders, in order.
function firstFile(nodes){
  const files = nodes.filter(n=>n.type==="file");
  const readme = files.find(n=>/^readme/i.test(n.name));
  if(readme) return readme.relpath;
  const landing = files.find(n=>/^(index|home)\b/i.test(n.name));
  if(landing) return landing.relpath;
  if(files.length) return files[0].relpath;
  for(const n of nodes){
    const f = firstFile(n.children||[]);
    if(f) return f;
  }
  return null;
}
function renderNodes(nodes){
  const frag = document.createDocumentFragment();
  for(const n of nodes){
    const node = document.createElement("div");
    node.className = "node";
    const row = document.createElement("div");
    row.className = "row";
    if(n.type==="dir"){
      node.classList.add("closed");   // folders start collapsed; the active doc's branch auto-opens
      row.innerHTML = '<span class="caret">'+ICONS.chevron+'</span><span class="ico"><span class="f-open">'+ICONS.folderOpen+'</span><span class="f-closed">'+ICONS.folder+'</span></span>';
      const nm = document.createElement("span"); nm.className="name"; nm.textContent=n.name;
      row.appendChild(nm);
      row.onclick = () => node.classList.toggle("closed");
      node.appendChild(row);
      const ch = document.createElement("div"); ch.className="children";
      ch.appendChild(renderNodes(n.children));
      node.appendChild(ch);
    } else {
      row.dataset.path = n.relpath;
      row.innerHTML = '<span class="caret"></span><span class="ico">'+ICONS.file+'</span>';
      const nm = document.createElement("span"); nm.className="name";
      nm.textContent = n.name.replace(/\.md$/i,"");
      row.appendChild(nm);
      row.onclick = () => openFile(n.relpath);
      node.appendChild(row);
    }
    frag.appendChild(node);
  }
  return frag;
}
function highlightActive(){
  document.querySelectorAll(".row.active").forEach(r=>r.classList.remove("active"));
  if(!state.path) return;
  const r = document.querySelector('.row[data-path="'+cssEsc(state.path)+'"]');
  if(r){ r.classList.add("active");
    let p = r.parentElement;
    while(p){ if(p.classList && p.classList.contains("node")) p.classList.remove("closed"); p=p.parentElement; }
  }
}
const cssEsc = s => s.replace(/["\\]/g, "\\$&");

/* ---------- open + render a markdown file ---------- */
async function openFile(relpath, keepScroll){
  if(state.path && state.path!==relpath) saveScrollNow();
  const prevRatio = keepScroll ? viewport.scrollTop / Math.max(1, viewport.scrollHeight) : 0;
  let md;
  try{ md = await (await fetch("/raw?path="+encodeURIComponent(relpath))).text(); }
  catch(e){ md = "# Could not load\n\n` + "`" + `"+relpath+"` + "`" + `"; }
  state.path = relpath;
  $("#empty")?.remove();
  page.style.display = "";
  page.innerHTML = marked.parse(md);
  enhance(relpath);
  buildOutline();
  highlightActive();
  crumb(relpath);
  layoutPage();
  applyTransform();
  document.title = relpath.split("/").pop().replace(/\.md$/i,"") + " — mdserve";
  location.hash = encodeURIComponent(relpath);
  localStorage.setItem("mdr-last", relpath);
  if(keepScroll) viewport.scrollTop = prevRatio * viewport.scrollHeight;
  else applyRatio(savedRatio(relpath));
  updateProgress(); updateFab();
  if($("#findbar").classList.contains("show")) runFind($("#find-in").value);
  setTimeout(()=>{ layoutPage(); applyTransform();
    if(!keepScroll) applyRatio(savedRatio(relpath));
    updateProgress(); updateFab(); }, 80);
}
function crumb(relpath){
  const parts = relpath.split("/");
  const file = parts.pop().replace(/\.md$/i,"");
  $("#crumb").innerHTML = (parts.length? parts.join(" / ")+" / ":"") + "<b>"+esc(file)+"</b>";
}
const esc = s => s.replace(/[&<>]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;"}[c]));

/* code blocks -> mermaid / highlight, headings, links, math, tables */
function enhance(relpath){
  page.querySelectorAll("pre > code").forEach(code=>{
    const cls = [...code.classList].find(c=>c.startsWith("language-"));
    const lang = cls ? cls.slice(9) : "";
    if(lang==="mermaid"){
      const div = document.createElement("div");
      div.className = "mermaid";
      div.textContent = code.textContent;
      code.parentElement.replaceWith(div);
    } else {
      try{ hljs.highlightElement(code); }catch(e){}
    }
  });
  const nodes = page.querySelectorAll(".mermaid");
  if(nodes.length){
    try{
      mermaid.initialize({startOnLoad:false, securityLevel:"loose",
        theme:mermaidTheme(state.theme),
        fontFamily:'-apple-system,Segoe UI,Roboto,sans-serif'});
      mermaid.run({nodes});
    }catch(e){ console.warn("mermaid", e); }
  }
  try{
    // KaTeX's $...$ inline delimiter would otherwise pair across currency
    // amounts (e.g. "$100 ... $5,000"), rendering prices as math. Neutralize
    // any $ immediately followed by a digit (a price, not math), render, restore.
    const PH="\u0000";
    mathWalk(page, s => s.replace(/(^|[^$])\$(?=\d)/g, "$1"+PH));
    renderMathInElement(page,{
      delimiters:[
        {left:"$$",right:"$$",display:true},
        {left:"\\[",right:"\\]",display:true},
        {left:"$",right:"$",display:false},
        {left:"\\(",right:"\\)",display:false}
      ],
      ignoredTags:["script","noscript","style","textarea","pre","code"],
      throwOnError:false
    });
    mathWalk(page, s => s.split(PH).join("$"));
  }catch(e){}
  const seen={};
  page.querySelectorAll("h1,h2,h3,h4").forEach(h=>{
    let id = slug(h.textContent);
    if(seen[id]!=null){ seen[id]++; id=id+"-"+seen[id]; } else seen[id]=0;
    h.id = id;
  });
  const dir = relpath.includes("/") ? relpath.slice(0,relpath.lastIndexOf("/")+1) : "";
  page.querySelectorAll("a[href]").forEach(a=>{
    const href = a.getAttribute("href");
    if(href.startsWith("#")){
      a.onclick = e=>{ e.preventDefault();
        const id = decodeURIComponent(href.slice(1));
        const t = document.getElementById(id)
               || [...page.querySelectorAll("[id]")].find(el=>el.id===id);
        scrollToEl(t);
      };
    } else if(/^[a-z]+:\/\//i.test(href) || href.startsWith("//") || href.startsWith("mailto:")){
      a.target="_blank"; a.rel="noopener noreferrer";
    } else if(/\.md(#.*)?$/i.test(href)){
      a.onclick = e=>{ e.preventDefault();
        const clean = href.split("#")[0];
        openFile(normalizePath(dir+clean)); };
    } else {
      a.setAttribute("href", fileURL(dir+href.split("#")[0]));
      a.target="_blank"; a.rel="noopener noreferrer";
    }
  });
  // local images / assets (png, svg, gif, …) → served from disk via /file,
  // resolved relative to this doc's directory; skip absolute / data / anchor srcs
  page.querySelectorAll("img[src]").forEach(img=>{
    const src=img.getAttribute("src");
    if(src && !/^([a-z]+:|\/\/|\/|data:|#)/i.test(src)) img.setAttribute("src", fileURL(dir+src));
  });
  page.querySelectorAll("table").forEach(t=>{
    if(t.parentElement && t.parentElement.classList.contains("table-wrap")) return;
    const w=document.createElement("div"); w.className="table-wrap";
    t.replaceWith(w); w.appendChild(t);
  });
}
const slug = t => t.toLowerCase().trim()
  .replace(/[^\w\s-]/g,"").replace(/\s+/g,"-").replace(/-+/g,"-") || "section";
function normalizePath(p){
  const out=[]; for(const seg of p.split("/")){
    if(seg===".."){ out.pop(); } else if(seg!=="." && seg!=="") out.push(seg);
  } return out.join("/");
}
// URL for a local asset (image, etc.) served by the backend, relative to a doc
const fileURL = p => "/file?path=" + encodeURIComponent(normalizePath(p));

/* scroll viewport so an element reaches the top — works through #page's transform */
function scrollToEl(el){
  if(!el) return;
  const r = el.getBoundingClientRect(), vr = viewport.getBoundingClientRect();
  const top = viewport.scrollTop + (r.top - vr.top) - 16;
  viewport.scrollTo({ top: Math.max(0, top), behavior: "smooth" });
}

/* ---------- responsive page width (fills the column, capped) ---------- */
const PAGE_MAX = 1040;
function layoutPage(){
  const avail = viewport.clientWidth;
  page.style.width = Math.max(320, Math.min(PAGE_MAX, avail - 40)) + "px";
}

/* ---------- per-doc scroll position (localStorage) ---------- */
const scrollRange = () => Math.max(1, viewport.scrollHeight - viewport.clientHeight);
const curRatio = () => viewport.scrollTop / scrollRange();
function applyRatio(r){ viewport.scrollTop = Math.max(0, (+r || 0) * scrollRange()); }
const scrollKey = p => "mdr-scroll:" + p;
function savedRatio(p){ const v = parseFloat(localStorage.getItem(scrollKey(p))); return isFinite(v) ? v : 0; }
let saveTimer = null;
function saveScroll(){
  if(!state.path) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(()=>{ if(state.path) localStorage.setItem(scrollKey(state.path), curRatio()); }, 300);
}
function saveScrollNow(){ if(state.path) localStorage.setItem(scrollKey(state.path), curRatio()); }

/* ---------- floating go-to-top ---------- */
function updateFab(){ $("#fab-up").classList.toggle("show", viewport.scrollTop > 240); }

/* ---------- resizable rails ---------- */
function makeResizable(railId, key, side){
  const rail=document.getElementById(railId);
  const h=rail.querySelector(".rail-resize"); if(!h) return;
  let st=null;
  h.addEventListener("mousedown",e=>{ st={x:e.clientX, w:rail.getBoundingClientRect().width};
    rail.style.transition="none"; h.classList.add("drag"); document.body.style.cursor="col-resize"; e.preventDefault(); });
  window.addEventListener("mousemove",e=>{ if(!st) return;
    const dx=e.clientX-st.x;
    let w = side==="left" ? st.w+dx : st.w-dx;
    rail.style.width = Math.max(170, Math.min(520, w)) + "px"; });
  window.addEventListener("mouseup",()=>{ if(!st) return; st=null; rail.style.transition="";
    h.classList.remove("drag"); document.body.style.cursor="";
    localStorage.setItem(key, parseInt(rail.style.width)||"");
    layoutPage(); applyTransform(); updateProgress(); });
}

/* ---------- in-page find (case-insensitive) ---------- */
let findMarks=[], findIdx=-1;
function clearFind(){
  findMarks.forEach(m=>{ const t=document.createTextNode(m.textContent); m.replaceWith(t); });
  if(findMarks.length) page.normalize();
  findMarks=[]; findIdx=-1; const c=$("#find-count"); if(c) c.textContent="0/0";
}
function runFind(q){
  clearFind(); q=(q||"").trim(); if(!q) return;
  const rx=new RegExp(q.replace(/[.*+?^${}()|[\]\\]/g,"\\$&"),"gi");
  const walker=document.createTreeWalker(page, NodeFilter.SHOW_TEXT, {
    acceptNode(n){
      if(!n.nodeValue || !n.nodeValue.trim()) return NodeFilter.FILTER_REJECT;
      const p=n.parentNode; if(!p || p.nodeType!==1) return NodeFilter.FILTER_REJECT;
      if(p.tagName==="MARK") return NodeFilter.FILTER_REJECT;
      if(p.closest(".mermaid,.katex,script,style")) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    }});
  const nodes=[]; let n; while(n=walker.nextNode()) nodes.push(n);
  nodes.forEach(node=>{
    const text=node.nodeValue; rx.lastIndex=0; if(!rx.test(text)) return;
    rx.lastIndex=0; const frag=document.createDocumentFragment(); let last=0, m;
    while((m=rx.exec(text))){
      if(m.index>last) frag.appendChild(document.createTextNode(text.slice(last,m.index)));
      const mk=document.createElement("mark"); mk.className="find"; mk.textContent=m[0];
      frag.appendChild(mk); findMarks.push(mk); last=m.index+m[0].length;
      if(m[0].length===0) rx.lastIndex++;
    }
    if(last<text.length) frag.appendChild(document.createTextNode(text.slice(last)));
    node.parentNode.replaceChild(frag, node);
  });
  if(findMarks.length){ findIdx=0; focusFind(); }
  else $("#find-count").textContent="0/0";
}
function focusFind(){
  findMarks.forEach(m=>m.classList.remove("cur"));
  const m=findMarks[findIdx]; if(!m) return;
  m.classList.add("cur"); scrollToEl(m);
  $("#find-count").textContent=(findIdx+1)+"/"+findMarks.length;
}
function findNext(d){ if(!findMarks.length) return; findIdx=(findIdx+d+findMarks.length)%findMarks.length; focusFind(); }
function openFind(){ $("#findbar").classList.add("show"); const i=$("#find-in"); i.focus(); i.select(); if(i.value) runFind(i.value); }
function closeFind(){ $("#findbar").classList.remove("show"); clearFind(); }

/* ---------- outline + scrollspy ---------- */
function buildOutline(){
  toc.innerHTML = "";
  const hs = [...page.querySelectorAll("h1,h2,h3")];
  if(!hs.length){ toc.innerHTML='<div style="padding:10px;color:var(--faint);font-size:12px">No headings</div>'; }
  hs.forEach(h=>{
    const a=document.createElement("a");
    a.textContent=h.textContent; a.href="#"+h.id;
    a.className = h.tagName==="H2"?"lvl2":h.tagName==="H3"?"lvl3":"";
    a.onclick=e=>{e.preventDefault(); scrollToEl(h);};
    toc.appendChild(a);
  });
  if(state.obs) state.obs.disconnect();
  state.obs = new IntersectionObserver(es=>{
    es.forEach(en=>{ if(en.isIntersecting) setActiveToc(en.target.id); });
  },{root:viewport, rootMargin:"0px 0px -75% 0px", threshold:0});
  hs.forEach(h=>state.obs.observe(h));
}
function setActiveToc(id){
  toc.querySelectorAll("a").forEach(a=>
    a.classList.toggle("active", a.getAttribute("href")==="#"+id));
}

/* ---------- empty state ---------- */
function showEmpty(){
  page.style.display="none";
  if($("#empty")) return;
  const d=document.createElement("div"); d.id="empty";
  d.innerHTML="<h2>No markdown here yet</h2><p>Point mdserve at a folder of <code>.md</code> files and they'll appear in the sidebar automatically.</p>";
  viewport.appendChild(d);
  $("#crumb").textContent=""; toc.innerHTML="";
}

/* ---------- zoom — transform:scale on #page, #canvas sized to match ---------- */
// At 100% the page carries no transform at all, so it scrolls on the browser's
// fast path instead of living on a permanently-rasterized compositor layer
// (a permanent will-change:transform made long docs janky). The GPU hint is
// added only while actively zooming, then dropped once idle.
let whTimer = null;
function hintTransform(){
  page.style.willChange = "transform";
  clearTimeout(whTimer);
  whTimer = setTimeout(()=>{ page.style.willChange = "auto"; }, 400);
}
function applyTransform(){
  page.style.transform = state.zoom === 1 ? "none" : "scale("+state.zoom+")";
  canvas.style.width  = (page.offsetWidth  * state.zoom) + "px";
  canvas.style.height = (page.offsetHeight * state.zoom) + "px";
  if(state.zoom !== 1) hintTransform();
}
function applyZoom(z){
  state.zoom = Math.min(3, Math.max(.4, z));
  applyTransform();
  $("#zoomlbl").textContent = Math.round(state.zoom*100)+"%";
}
function zoomTo(z, cx, cy){
  z = Math.min(3, Math.max(.4, z));
  const vr = viewport.getBoundingClientRect();
  const ax = (cx==null ? vr.left + viewport.clientWidth/2  : cx);
  const ay = (cy==null ? vr.top  + viewport.clientHeight/2 : cy);
  const pr = page.getBoundingClientRect();
  const withinX = (ax - pr.left) / state.zoom;
  const withinY = (ay - pr.top ) / state.zoom;
  state.zoom = z;
  applyTransform();
  $("#zoomlbl").textContent = Math.round(z*100)+"%";
  const pr2 = page.getBoundingClientRect();
  const prevSB = viewport.style.scrollBehavior;
  viewport.style.scrollBehavior = "auto";
  viewport.scrollLeft += (pr2.left + withinX*z) - ax;
  viewport.scrollTop  += (pr2.top  + withinY*z) - ay;
  viewport.style.scrollBehavior = prevSB;
  localStorage.setItem("mdr-zoom", z);
  updateProgress();
}

/* ---------- reading progress ---------- */
function updateProgress(){
  const max = viewport.scrollHeight - viewport.clientHeight;
  $("#progress").style.width = (max>0 ? (viewport.scrollTop/max)*100 : 0) + "%";
}
viewport.addEventListener("scroll", ()=>{ updateProgress(); updateFab(); saveScroll(); }, {passive:true});
let resizeTimer=null;
window.addEventListener("resize", ()=>{ clearTimeout(resizeTimer);
  resizeTimer=setTimeout(()=>{ layoutPage(); applyTransform(); updateProgress(); }, 120); });

/* ---------- collapsible rails (persisted) ---------- */
// the sidebar button shows left-panel-close while open, left-panel-open while collapsed
function updSide(){
  const b=$("#b-side"); if(!b) return;
  const collapsed=$("#sidebar").classList.contains("collapsed");
  b.innerHTML=MS(collapsed?PATH.panelOpen:PATH.panelClose);
  b.title=(collapsed?"Show":"Hide")+" file list (Cmd/Ctrl B)";
}
function toggleRail(id, key){
  const collapsed = document.getElementById(id).classList.toggle("collapsed");
  localStorage.setItem(key, collapsed ? "1" : "0");
  if(id==="sidebar") updSide();
  setTimeout(()=>{ layoutPage(); applyTransform(); updateProgress(); }, 220);
}

/* ---------- sidebar filter ---------- */
function filterTree(q){
  q = q.trim().toLowerCase();
  const tree = $("#tree");
  tree.querySelector(".empty-filter")?.remove();
  const nodes = tree.querySelectorAll(".node");
  if(!q){ nodes.forEach(n=>n.classList.remove("hide")); return; }
  nodes.forEach(n=>n.classList.add("hide"));
  let hits = 0;
  tree.querySelectorAll('.row[data-path]').forEach(row=>{
    if(row.querySelector(".name").textContent.toLowerCase().includes(q)){
      hits++;
      let n = row.parentElement;
      while(n && n !== tree){
        if(n.classList && n.classList.contains("node")){
          n.classList.remove("hide"); n.classList.remove("closed");
        }
        n = n.parentElement;
      }
    }
  });
  if(!hits){ const d=document.createElement("div"); d.className="empty-filter";
    d.textContent="No docs match “"+q+"”"; tree.appendChild(d); }
}

/* ---------- toolbar wiring ---------- */
$("#b-zin").onclick   = ()=>zoomTo(state.zoom*1.15);
$("#b-zout").onclick  = ()=>zoomTo(state.zoom/1.15);
$("#b-zreset").onclick= ()=>zoomTo(1);
$("#b-side").onclick  = ()=>toggleRail("sidebar","mdr-side");
$("#b-out").onclick   = ()=>toggleRail("outline","mdr-out");
$("#b-print").onclick = ()=>{
  const prev=state.theme; let done=false;
  const restore=()=>{ if(done) return; done=true;
    if(prev!=="light") setTheme(prev);
    window.removeEventListener("afterprint", restore); };
  if(prev!=="light") setTheme("light");
  window.addEventListener("afterprint", restore);
  setTimeout(()=>window.print(), 90);
  setTimeout(restore, 8000);
};
$("#b-select").onclick= ()=>setTool("select");
$("#b-hand").onclick  = ()=>setTool("hand");
$("#b-find").onclick  = ()=>($("#findbar").classList.contains("show") ? closeFind() : openFind());
$("#fab-up").onclick  = ()=>viewport.scrollTo({top:0, behavior:"smooth"});
$("#find-next").onclick=()=>findNext(1);
$("#find-prev").onclick=()=>findNext(-1);
$("#find-close").onclick=closeFind;
$("#find-in").addEventListener("input", e=>runFind(e.target.value));
$("#find-in").addEventListener("keydown", e=>{
  if(e.key==="Enter"){ e.preventDefault(); findNext(e.shiftKey?-1:1); }
  else if(e.key==="Escape"){ e.preventDefault(); closeFind(); }
});
$("#b-theme").onclick = ()=>{
  const i=THEMES.indexOf(state.theme); setTheme(THEMES[(i+1)%THEMES.length]);
};
// reading font — toggle the #page body type between serif (default) and sans
const FONTS = ["serif","sans"];
function setFont(f){
  document.body.dataset.font = f; localStorage.setItem("mdr-font", f);
  const b=$("#b-font"); if(b){ b.classList.toggle("on", f==="serif"); b.title="Reading font: "+f+" — click for "+(f==="serif"?"sans":"serif"); }
}
$("#b-font").onclick = ()=>{ const i=FONTS.indexOf(document.body.dataset.font||"serif"); setFont(FONTS[(i+1)%FONTS.length]); };
// fullscreen toggle (icon swaps enter/exit)
function updFull(){
  const on=!!document.fullscreenElement, b=$("#b-full");
  if(b){ b.innerHTML=MS(on?PATH.fullExit:PATH.full); b.classList.toggle("on",on); b.title=on?"Exit full screen":"Full screen"; }
}
$("#b-full").onclick = ()=>{ if(document.fullscreenElement) document.exitFullscreen(); else if(document.documentElement.requestFullscreen) document.documentElement.requestFullscreen(); };
document.addEventListener("fullscreenchange", updFull);
const filterEl = $("#filter");
filterEl.addEventListener("input", e=>{
  $(".filter").classList.toggle("has", !!e.target.value);
  filterTree(e.target.value);
});
filterEl.addEventListener("keydown", e=>{ if(e.key==="Escape"){ filterEl.value=""; $(".filter").classList.remove("has"); filterTree(""); filterEl.blur(); }});
$("#b-clr").onclick = ()=>{ filterEl.value=""; $(".filter").classList.remove("has"); filterTree(""); filterEl.focus(); };
function setTool(t){
  state.tool=t;
  viewport.classList.toggle("hand", t==="hand");
  $("#b-hand").classList.toggle("on", t==="hand");
  $("#b-select").classList.toggle("on", t==="select");
}
// favicon recolors with the theme (brown on warm, blue otherwise) to match the
// menubar logo, which is themed via --logo.
const FAVCOL = {dark:"#2f81f7", light:"#0969da", warm:"#b5651d"};
function favSvg(c){
  return "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='7' fill='"+encodeURIComponent(c)+"'/%3E%3Cpath d='M9 11h14M9 16h14M9 21h9' fill='none' stroke='white' stroke-width='2.4' stroke-linecap='round'/%3E%3C/svg%3E";
}
function setFavicon(t){ const l=document.querySelector('link[rel="icon"]'); if(l) l.href=favSvg(FAVCOL[t]||FAVCOL.dark); }
function setTheme(t){
  state.theme=t; document.body.dataset.theme=t;
  localStorage.setItem("mdr-theme", t);
  setFavicon(t);
  // swap the highlight.js stylesheet so code/markdown highlighting is crisp in
  // each mode (a real dark theme for dark, the light theme for light/warm)
  const hl=document.getElementById("hljs-theme");
  if(hl) hl.href = t==="dark" ? "/vendor/hljs-dark.css" : "/vendor/hljs-theme.css";
  const b=$("#b-theme");
  if(b){ b.innerHTML=ICONS.theme[t]; b.title="Theme: "+t+" (click to change)"; }
}

/* ---------- hand / pan drag ---------- */
let drag=null;
viewport.addEventListener("mousedown",e=>{
  if(state.tool!=="hand" && e.button!==1) return;
  drag={x:e.clientX,y:e.clientY,l:viewport.scrollLeft,t:viewport.scrollTop};
  viewport.style.scrollBehavior="auto";
  viewport.classList.add("grabbing"); e.preventDefault();
});
window.addEventListener("mousemove",e=>{
  if(!drag) return;
  e.preventDefault();
  viewport.scrollLeft = drag.l-(e.clientX-drag.x);
  viewport.scrollTop  = drag.t-(e.clientY-drag.y);
});
window.addEventListener("mouseup",()=>{
  if(!drag) return;
  drag=null; viewport.style.scrollBehavior=""; viewport.classList.remove("grabbing");
});

/* ctrl/cmd + wheel zoom — anchored at the cursor */
viewport.addEventListener("wheel",e=>{
  if(e.ctrlKey||e.metaKey){ e.preventDefault();
    zoomTo(state.zoom * Math.exp(-e.deltaY*0.0015), e.clientX, e.clientY); }
},{passive:false});

/* keyboard */
window.addEventListener("keydown",e=>{
  const mod=e.ctrlKey||e.metaKey;
  if(mod && (e.key==="="||e.key==="+")){ e.preventDefault(); zoomTo(state.zoom*1.15); }
  else if(mod && e.key==="-"){ e.preventDefault(); zoomTo(state.zoom/1.15); }
  else if(mod && e.key==="0"){ e.preventDefault(); zoomTo(1); }
  else if(mod && e.key.toLowerCase()==="b"){ e.preventDefault(); toggleRail("sidebar","mdr-side"); }
  else if(mod && e.key==="\\"){ e.preventDefault(); toggleRail("outline","mdr-out"); }
  else if(mod && e.key.toLowerCase()==="f"){ e.preventDefault(); openFind(); }
  else if(e.key==="Escape" && $("#findbar").classList.contains("show")){ closeFind(); }
  else if(e.key==="/" && !inField()){ e.preventDefault(); $("#filter").focus(); }
  else if(e.key==="h" && !mod && !inField()){ setTool(state.tool==="hand"?"select":"hand"); }
});
const inField = ()=>/^(input|textarea|select)$/i.test(document.activeElement.tagName);

/* ---------- auto-reload polling ---------- */
async function poll(){
  try{
    const m = await (await fetch("/api/poll")).json();
    const oldKeys = Object.keys(state.mtimes).sort().join("|");
    const newKeys = Object.keys(m).sort().join("|");
    if(oldKeys !== newKeys) await loadTree();
    if(state.path && m[state.path] && m[state.path] !== state.mtimes[state.path]){
      await openFile(state.path, true);
    }
    state.mtimes = m;
  }catch(e){}
}

/* ---------- boot ---------- */
(function init(){
  setTheme(localStorage.getItem("mdr-theme") || "dark");
  setFont(localStorage.getItem("mdr-font") || "sans");
  updFull();
  if(localStorage.getItem("mdr-side")==="1") $("#sidebar").classList.add("collapsed");
  if(localStorage.getItem("mdr-out")==="1")  $("#outline").classList.add("collapsed");
  updSide();
  const sw=parseInt(localStorage.getItem("mdr-sideW")); if(sw) $("#sidebar").style.width=sw+"px";
  const ow=parseInt(localStorage.getItem("mdr-outW")); if(ow) $("#outline").style.width=ow+"px";
  makeResizable("sidebar","mdr-sideW","left");
  makeResizable("outline","mdr-outW","right");
  layoutPage();
  const z0 = parseFloat(localStorage.getItem("mdr-zoom"));
  if(z0 && z0>0) applyZoom(z0);
  bootTarget = (location.hash ? decodeURIComponent(location.hash.slice(1)) : "")
             || localStorage.getItem("mdr-last")
             || (window.MDSERVE && window.MDSERVE.defaultDoc) || "";
  loadTree().then(()=>{
    fetch("/api/poll").then(r=>r.json()).then(m=>state.mtimes=m).catch(()=>{});
  });
  if(!(window.MDSERVE && window.MDSERVE.reload===false)) setInterval(poll, 2000);
})();
</script>
</body>
</html>`

// buildShell is the standalone page emitted by ` + "`mdserve build`" + ` for each doc:
// server-rendered Markdown wrapped in the same theme, with the embedded vendor
// bundle copied alongside so the static site is fully offline too. Sentinels
// __ROOT__ (relative path to the output root), __TITLE__ and __BODY__ are
// substituted per file.
const buildShell = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>__TITLE__ — mdserve</title>
<link rel="stylesheet" href="__ROOT__vendor/hljs-theme.css">
<link rel="stylesheet" href="__ROOT__vendor/katex.min.css">
<style>
[data-theme="warm"]{--bg:#fbfaf6;--ink:#34352f;--muted:#8c8b80;--border:#e5e2d8;--accent:#8a7f6a;--accent-soft:#cdc8ba;--code-bg:#f1efe8;--code-ink:#56534a;--hljs-filter:none}
*{box-sizing:border-box}
body{margin:0;background:#edece6;color:var(--ink);
  font-family:"Iowan Old Style","Palatino Linotype",Palatino,Georgia,serif;line-height:1.68}
#page{max-width:820px;margin:0 auto;background:var(--bg);min-height:100vh;padding:48px 56px 96px}
#page h1,#page h2,#page h3,#page h4{line-height:1.25;font-weight:700;font-family:-apple-system,"Segoe UI",sans-serif}
#page h1{font-size:2.05em;border-bottom:2px solid var(--border);padding-bottom:.28em}
#page h2{font-size:1.5em;border-bottom:1px solid var(--border);padding-bottom:.22em;margin-top:1.7em}
#page a{color:var(--accent)}
#page pre{background:var(--code-bg);border:1px solid var(--border);border-radius:11px;padding:16px 18px;overflow:auto}
#page code{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:.86em;background:var(--code-bg);color:var(--code-ink);padding:.15em .42em;border-radius:5px}
#page pre code{background:none;padding:0;filter:var(--hljs-filter)}
#page table{border-collapse:collapse;font-size:.92em;font-family:-apple-system,sans-serif}
#page th,#page td{border:1px solid var(--border);padding:8px 13px;text-align:left}
#page th{background:var(--code-bg)}
#page blockquote{margin:1.2em 0;padding:.5em 1.1em;border-left:4px solid var(--accent-soft);color:var(--muted)}
#page img{max-width:100%}
#page .mermaid{text-align:center}
</style>
</head>
<body data-theme="warm">
<main id="page">__BODY__</main>
<script src="__ROOT__vendor/highlight.min.js"></script>
<script src="__ROOT__vendor/mermaid.min.js"></script>
<script src="__ROOT__vendor/katex.min.js"></script>
<script src="__ROOT__vendor/auto-render.min.js"></script>
<script>
document.querySelectorAll("pre>code").forEach(function(c){
  if([...c.classList].some(x=>x==="language-mermaid")){
    var d=document.createElement("div"); d.className="mermaid"; d.textContent=c.textContent;
    c.closest("pre").replaceWith(d);
  } else { try{ hljs.highlightElement(c); }catch(e){} }
});
try{ mermaid.initialize({startOnLoad:true,securityLevel:"loose",theme:"neutral"}); }catch(e){}
try{ (function(){
  var page=document.getElementById("page"), PH="\u0000";
  // shield currency ($100) from KaTeX's $...$ delimiter, then restore it
  function walk(fn){
    var w=document.createTreeWalker(page,NodeFilter.SHOW_TEXT,{acceptNode:function(n){
      return n.parentElement&&n.parentElement.closest("pre,code,script,style,textarea,.katex,.mermaid")
        ?NodeFilter.FILTER_REJECT:NodeFilter.FILTER_ACCEPT;}});
    var out=[],n; while(n=w.nextNode()) out.push(n);
    out.forEach(function(t){ var v=fn(t.nodeValue); if(v!==t.nodeValue) t.nodeValue=v; });
  }
  walk(function(s){return s.replace(/(^|[^$])\$(?=\d)/g,"$1"+PH);});
  renderMathInElement(page,{delimiters:[
    {left:"$$",right:"$$",display:true},{left:"\\[",right:"\\]",display:true},
    {left:"$",right:"$",display:false},{left:"\\(",right:"\\)",display:false}],
    ignoredTags:["script","noscript","style","textarea","pre","code"],throwOnError:false});
  walk(function(s){return s.split(PH).join("$");});
})(); }catch(e){}
</script>
</body>
</html>`
