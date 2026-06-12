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
<link rel="stylesheet" href="/vendor/hljs-theme.css">
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
  --app-bg:#16181c; --rail-bg:#1c1f25; --paper:#22262d; --ink:#dde2e9;
  --muted:#98a1ad; --faint:#5f6a77; --accent:#a7b0bd; --accent-soft:#3a414c;
  --active:#2a2f37; --border:#2b313a; --code-bg:#191c22; --code-ink:#c6cfdb; --sel:#333b46; --sel-strong:#2f81f7; --logo:#2f81f7;
  --shadow:0 1px 2px rgba(0,0,0,.3),0 10px 30px rgba(0,0,0,.45); --hljs-filter:invert(.92) hue-rotate(180deg);
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
  font-size:16px; line-height:1.68; transform-origin:0 0; will-change:transform;
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
#page pre code{background:none;padding:0;font-size:13px;filter:var(--hljs-filter)}
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
<body data-theme="dark" data-font="serif">

<aside id="sidebar">
  <div id="brand"><svg class="logo" viewBox="0 0 32 32"><rect width="32" height="32" rx="7" fill="#2f81f7"/><path d="M9 11h14M9 16h14M9 21h9" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/></svg><span class="name">mdserve</span><span class="cwd" id="rootname"></span></div>
  <div class="rail-top">
    <div class="filter">
      <span class="sicon"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M440-160q-17 0-28.5-11.5T400-200v-240L168-736q-15-20-4.5-42t36.5-22h560q26 0 36.5 22t-4.5 42L560-440v240q0 17-11.5 28.5T520-160h-80Zm40-308 198-252H282l198 252Zm0 0Z"/></svg></span>
      <input id="filter" type="text" placeholder="Filter files  ( / )" spellcheck="false" autocomplete="off">
      <button class="clr" id="b-clr" title="Clear"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-424 284-228q-11 11-28 11t-28-11q-11-11-11-28t11-28l196-196-196-196q-11-11-11-28t11-28q11-11 28-11t28 11l196 196 196-196q11-11 28-11t28 11q11 11 11 28t-11 28L536-480l196 196q11 11 11 28t-11 28q-11 11-28 11t-28-11L480-424Z"/></svg></button>
    </div>
  </div>
  <div id="tree"></div>
  <div class="rail-resize" title="Drag to resize"></div>
</aside>

<section id="main">
  <div id="toolbar">
    <button class="tbtn" id="b-side" title="Toggle file list (Cmd/Ctrl B)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M660-368v-224q0-14-12-19t-22 5l-98 98q-12 12-12 28t12 28l98 98q10 10 22 5t12-19ZM200-120q-33 0-56.5-23.5T120-200v-560q0-33 23.5-56.5T200-840h560q33 0 56.5 23.5T840-760v560q0 33-23.5 56.5T760-120H200Zm120-80v-560H200v560h120Zm80 0h360v-560H400v560Zm-80 0H200h120Z"/></svg></button>
    <div class="sep"></div>
    <button class="tbtn" id="b-zout" title="Zoom out (Ctrl -)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M320-540q-17 0-28.5-11.5T280-580q0-17 11.5-28.5T320-620h120q17 0 28.5 11.5T480-580q0 17-11.5 28.5T440-540H320Zm60 220q-109 0-184.5-75.5T120-580q0-109 75.5-184.5T380-840q109 0 184.5 75.5T640-580q0 44-14 83t-38 69l224 224q11 11 11 28t-11 28q-11 11-28 11t-28-11L532-372q-30 24-69 38t-83 14Zm0-80q75 0 127.5-52.5T560-580q0-75-52.5-127.5T380-760q-75 0-127.5 52.5T200-580q0 75 52.5 127.5T380-400Z"/></svg></button>
    <span id="zoomlbl">100%</span>
    <button class="tbtn" id="b-zin" title="Zoom in (Ctrl +)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M340-540h-40q-17 0-28.5-11.5T260-580q0-17 11.5-28.5T300-620h40v-40q0-17 11.5-28.5T380-700q17 0 28.5 11.5T420-660v40h40q17 0 28.5 11.5T500-580q0 17-11.5 28.5T460-540h-40v40q0 17-11.5 28.5T380-460q-17 0-28.5-11.5T340-500v-40Zm40 220q-109 0-184.5-75.5T120-580q0-109 75.5-184.5T380-840q109 0 184.5 75.5T640-580q0 44-14 83t-38 69l224 224q11 11 11 28t-11 28q-11 11-28 11t-28-11L532-372q-30 24-69 38t-83 14Zm0-80q75 0 127.5-52.5T560-580q0-75-52.5-127.5T380-760q-75 0-127.5 52.5T200-580q0 75 52.5 127.5T380-400Z"/></svg></button>
    <button class="tbtn" id="b-zreset" title="Reset zoom to 100% (Ctrl 0)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M393-132q-103-29-168-113.5T160-440q0-57 19-108.5t54-94.5q11-12 27-12.5t29 12.5q11 11 11.5 27T290-586q-24 31-37 68t-13 78q0 81 47.5 144.5T410-209q13 4 21.5 15t8.5 24q0 20-14 31.5t-33 6.5Zm174 0q-19 5-33-7t-14-32q0-12 8.5-23t21.5-15q75-24 122.5-87T720-440q0-100-70-170t-170-70h-3l16 16q11 11 11 28t-11 28q-11 11-28 11t-28-11l-84-84q-6-6-8.5-13t-2.5-15q0-8 2.5-15t8.5-13l84-84q11-11 28-11t28 11q11 11 11 28t-11 28l-16 16h3q134 0 227 93t93 227q0 109-65 194T567-132Z"/></svg></button>
    <div class="sep"></div>
    <button class="tbtn on" id="b-select" title="Select text"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M606-105q-23 11-46 2.5T526-134L406-392l-93 130q-17 24-45 15t-28-38v-513q0-25 22.5-36t42.5 5l404 318q23 17 13.5 44T684-440H516l119 255q11 23 2.5 46T606-105Z"/></svg></button>
    <button class="tbtn" id="b-hand" title="Hand tool — drag to pan (H)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M402-40q-30 0-56-13.5T303-92L67-438q-8-12-7-26t12-24q19-19 45-22t47 12l116 81v-383q0-17 11.5-28.5T320-840q17 0 28.5 11.5T360-800v320h80v-400q0-17 11.5-28.5T480-920q17 0 28.5 11.5T520-880v400h80v-360q0-17 11.5-28.5T640-880q17 0 28.5 11.5T680-840v360h80v-280q0-17 11.5-28.5T800-800q17 0 28.5 11.5T840-760v560q0 66-47 113T680-40H402Z"/></svg></button>
    <div class="sep"></div>
    <button class="tbtn" id="b-theme" title="Cycle theme (warm / light / dark)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-360q50 0 85-35t35-85q0-50-35-85t-85-35q-50 0-85 35t-35 85q0 50 35 85t85 35Z"/></svg></button>
    <button class="tbtn" id="b-font" title="Reading font (serif / sans)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M340-160q-25 0-42.5-17.5T280-220v-460H140q-25 0-42.5-17.5T80-740q0-25 17.5-42.5T140-800h400q25 0 42.5 17.5T600-740q0 25-17.5 42.5T540-680H400v460q0 25-17.5 42.5T340-160Zm360 0q-25 0-42.5-17.5T640-220v-260h-60q-25 0-42.5-17.5T520-540q0-25 17.5-42.5T580-600h240q25 0 42.5 17.5T880-540q0 25-17.5 42.5T820-480h-60v260q0 25-17.5 42.5T700-160Z"/></svg></button>
    <button class="tbtn" id="b-full" title="Full screen"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M200-200h80q17 0 28.5 11.5T320-160q0 17-11.5 28.5T280-120H160q-17 0-28.5-11.5T120-160v-120q0-17 11.5-28.5T160-320q17 0 28.5 11.5T200-280v80Zm560 0v-80q0-17 11.5-28.5T800-320q17 0 28.5 11.5T840-280v120q0 17-11.5 28.5T800-120H680q-17 0-28.5-11.5T640-160q0-17 11.5-28.5T680-200h80ZM200-760v80q0 17-11.5 28.5T160-640q-17 0-28.5-11.5T120-680v-120q0-17 11.5-28.5T160-840h120q17 0 28.5 11.5T320-800q0 17-11.5 28.5T280-760h-80Zm560 0h-80q-17 0-28.5-11.5T640-800q0-17 11.5-28.5T680-840h120q17 0 28.5 11.5T840-800v120q0 17-11.5 28.5T800-640q-17 0-28.5-11.5T760-680v-80Z"/></svg></button>
    <button class="tbtn" id="b-print" title="Print / save as PDF"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M320-120q-33 0-56.5-23.5T240-200v-80h-80q-33 0-56.5-23.5T80-360v-160q0-51 35-85.5t85-34.5h560q51 0 85.5 34.5T880-520v160q0 33-23.5 56.5T800-280h-80v80q0 33-23.5 56.5T640-120H320Zm400-560H240v-80q0-33 23.5-56.5T320-840h320q33 0 56.5 23.5T720-760v80Zm0 220q17 0 28.5-11.5T760-500q0-17-11.5-28.5T720-540q-17 0-28.5 11.5T680-500q0 17 11.5 28.5T720-460ZM320-200h320v-160H320v160Z"/></svg></button>
    <button class="tbtn" id="b-find" title="Find in page (Cmd/Ctrl F)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M380-320q-109 0-184.5-75.5T120-580q0-109 75.5-184.5T380-840q109 0 184.5 75.5T640-580q0 44-14 83t-38 69l224 224q11 11 11 28t-11 28q-11 11-28 11t-28-11L532-372q-30 24-69 38t-83 14Zm0-80q75 0 127.5-52.5T560-580q0-75-52.5-127.5T380-760q-75 0-127.5 52.5T200-580q0 75 52.5 127.5T380-400Z"/></svg></button>
    <div id="crumb"></div>
    <button class="tbtn" id="b-out" title="Toggle outline (Cmd/Ctrl \\)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M160-280q-17 0-28.5-11.5T120-320q0-17 11.5-28.5T160-360h480q17 0 28.5 11.5T680-320q0 17-11.5 28.5T640-280H160Zm0-160q-17 0-28.5-11.5T120-480q0-17 11.5-28.5T160-520h480q17 0 28.5 11.5T680-480q0 17-11.5 28.5T640-440H160Zm0-160q-17 0-28.5-11.5T120-640q0-17 11.5-28.5T160-680h480q17 0 28.5 11.5T680-640q0 17-11.5 28.5T640-600H160Zm640 320q-17 0-28.5-11.5T760-320q0-17 11.5-28.5T800-360q17 0 28.5 11.5T840-320q0 17-11.5 28.5T800-280Zm0-160q-17 0-28.5-11.5T760-480q0-17 11.5-28.5T800-520q17 0 28.5 11.5T840-480q0 17-11.5 28.5T800-440Zm0-160q-17 0-28.5-11.5T760-640q0-17 11.5-28.5T800-680q17 0 28.5 11.5T840-640q0 17-11.5 28.5T800-600Z"/></svg></button>
  </div>
  <div id="progress"></div>
  <div id="findbar">
    <input id="find-in" type="text" placeholder="Find in page" spellcheck="false" autocomplete="off">
    <span id="find-count">0/0</span>
    <button class="tbtn" id="find-prev" title="Previous (Shift+Enter)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-528 324-372q-11 11-28 11t-28-11q-11-11-11-28t11-28l184-184q12-12 28-12t28 12l184 184q11 11 11 28t-11 28q-11 11-28 11t-28-11L480-528Z"/></svg></button>
    <button class="tbtn" id="find-next" title="Next (Enter)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-361q-8 0-15-2.5t-13-8.5L268-556q-11-11-11-28t11-28q11-11 28-11t28 11l156 156 156-156q11-11 28-11t28 11q11 11 11 28t-11 28L508-372q-6 6-13 8.5t-15 2.5Z"/></svg></button>
    <button class="tbtn" id="find-close" title="Close (Esc)"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M480-424 284-228q-11 11-28 11t-28-11q-11-11-11-28t11-28l196-196-196-196q-11-11-11-28t11-28q11-11 28-11t28 11l196 196 196-196q11-11 28-11t28 11q11 11 11 28t-11 28L536-480l196 196q11 11 11 28t-11 28q-11 11-28 11t-28-11L480-424Z"/></svg></button>
  </div>
  <div id="viewport">
    <div id="canvas"><div id="page"></div></div>
  </div>
  <button id="fab-up" title="Go to top"><svg viewBox="0 -960 960 960" fill="currentColor"><path d="M440-647 244-451q-12 12-28 11.5T188-452q-11-12-11.5-28t11.5-28l264-264q6-6 13-8.5t15-2.5q8 0 15 2.5t13 8.5l264 264q11 11 11 27.5T772-452q-12 12-28.5 12T715-452L520-647v447q0 17-11.5 28.5T480-160q-17 0-28.5-11.5T440-200v-447Z"/></svg></button>
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
  expandMore: 'M480-362q-8 0-15-2.5t-13-8.5L268-557q-11-11-11-28t11-28q11-11 28-11t28 11l156 156 156-156q11-11 28-11t28 11q11 11 11 28t-11 28L508-373q-6 6-13 8.5t-15 2.5Z',
  folder: 'M160-160q-33 0-56.5-23.5T80-240v-480q0-33 23.5-56.5T160-800h207q16 0 30.5 6t25.5 17l57 57h320q33 0 56.5 23.5T880-640v400q0 33-23.5 56.5T800-160H160Zm0-80h640v-400H447l-80-80H160v480Zm0 0v-480 480Z',
  folderOpen: 'M160-160q-33 0-56.5-23.5T80-240v-480q0-33 23.5-56.5T160-800h207q16 0 30.5 6t25.5 17l57 57h360q17 0 28.5 11.5T880-680q0 17-11.5 28.5T840-640H447l-80-80H160v480l79-263q8-26 29.5-41.5T316-560h516q41 0 64.5 32.5T909-457l-72 240q-8 26-29.5 41.5T760-160H160Zm84-80h516l72-240H316l-72 240Zm-84-262v-218 218Zm84 262 72-240-72 240Z',
  file: 'M360-240h240q17 0 28.5-11.5T640-280q0-17-11.5-28.5T600-320H360q-17 0-28.5 11.5T320-280q0 17 11.5 28.5T360-240Zm0-160h240q17 0 28.5-11.5T640-440q0-17-11.5-28.5T600-480H360q-17 0-28.5 11.5T320-440q0 17 11.5 28.5T360-400ZM240-80q-33 0-56.5-23.5T160-160v-640q0-33 23.5-56.5T240-880h287q16 0 30.5 6t25.5 17l194 194q11 11 17 25.5t6 30.5v447q0 33-23.5 56.5T720-80H240Zm280-560v-160H240v640h480v-440H560q-17 0-28.5-11.5T520-640ZM240-800v200-200 640-640Z',
  full: 'M200-200h80q17 0 28.5 11.5T320-160q0 17-11.5 28.5T280-120H160q-17 0-28.5-11.5T120-160v-120q0-17 11.5-28.5T160-320q17 0 28.5 11.5T200-280v80Zm560 0v-80q0-17 11.5-28.5T800-320q17 0 28.5 11.5T840-280v120q0 17-11.5 28.5T800-120H680q-17 0-28.5-11.5T640-160q0-17 11.5-28.5T680-200h80ZM200-760v80q0 17-11.5 28.5T160-640q-17 0-28.5-11.5T120-680v-120q0-17 11.5-28.5T160-840h120q17 0 28.5 11.5T320-800q0 17-11.5 28.5T280-760h-80Zm560 0h-80q-17 0-28.5-11.5T640-800q0-17 11.5-28.5T680-840h120q17 0 28.5 11.5T840-800v120q0 17-11.5 28.5T800-640q-17 0-28.5-11.5T760-680v-80Z',
  fullExit: 'M240-240h-80q-17 0-28.5-11.5T120-280q0-17 11.5-28.5T160-320h120q17 0 28.5 11.5T320-280v120q0 17-11.5 28.5T280-120q-17 0-28.5-11.5T240-160v-80Zm480 0v80q0 17-11.5 28.5T680-120q-17 0-28.5-11.5T640-160v-120q0-17 11.5-28.5T680-320h120q17 0 28.5 11.5T840-280q0 17-11.5 28.5T800-240h-80ZM240-720v-80q0-17 11.5-28.5T280-840q17 0 28.5 11.5T320-800v120q0 17-11.5 28.5T280-640H160q-17 0-28.5-11.5T120-680q0-17 11.5-28.5T160-720h80Zm480 0h80q17 0 28.5 11.5T840-680q0 17-11.5 28.5T800-640H680q-17 0-28.5-11.5T640-680v-120q0-17 11.5-28.5T680-840q17 0 28.5 11.5T720-800v80Z',
  warm: 'M792-670q11 11 11 28t-11 28l-29 29q-12 12-28.5 12T706-585q-12-12-11.5-28.5T707-642l29-29q12-11 28.5-10.5T792-670ZM120-160q-17 0-28.5-11.5T80-200q0-17 11.5-28.5T120-240h720q17 0 28.5 11.5T880-200q0 17-11.5 28.5T840-160H120Zm360-640q17 0 28.5 11.5T520-760v40q0 17-11.5 28.5T480-680q-17 0-28.5-11.5T440-720v-40q0-17 11.5-28.5T480-800ZM170-672q11-11 28-11t28 11l29 29q12 12 12 28.5T255-586q-12 11-29 11t-28-12l-29-29q-11-12-10.5-28.5T170-672Zm127 272h366q-23-54-72-87t-111-33q-62 0-111 33t-72 87Zm-97 80q0-117 81.5-198.5T480-600q117 0 198.5 81.5T760-320H200Zm280-80Z',
  light: 'M480-360q50 0 85-35t35-85q0-50-35-85t-85-35q-50 0-85 35t-35 85q0 50 35 85t85 35Zm0 80q-83 0-141.5-58.5T280-480q0-83 58.5-141.5T480-680q83 0 141.5 58.5T680-480q0 83-58.5 141.5T480-280ZM80-440q-17 0-28.5-11.5T40-480q0-17 11.5-28.5T80-520h80q17 0 28.5 11.5T200-480q0 17-11.5 28.5T160-440H80Zm720 0q-17 0-28.5-11.5T760-480q0-17 11.5-28.5T800-520h80q17 0 28.5 11.5T920-480q0 17-11.5 28.5T880-440h-80ZM480-760q-17 0-28.5-11.5T440-800v-80q0-17 11.5-28.5T480-920q17 0 28.5 11.5T520-880v80q0 17-11.5 28.5T480-760Zm0 720q-17 0-28.5-11.5T440-80v-80q0-17 11.5-28.5T480-200q17 0 28.5 11.5T520-160v80q0 17-11.5 28.5T480-40ZM226-678l-43-42q-12-11-11.5-28t11.5-29q12-12 29-12t28 12l42 43q11 12 11 28t-11 28q-11 12-27.5 11.5T226-678Zm494 495-42-43q-11-12-11-28.5t11-27.5q11-12 27.5-11.5T734-282l43 42q12 11 11.5 28T777-183q-12 12-29 12t-28-12Zm-42-495q-12-11-11.5-27.5T678-734l42-43q11-12 28-11.5t29 11.5q12 12 12 29t-12 28l-43 42q-12 11-28 11t-28-11ZM183-183q-12-12-12-29t12-28l43-42q12-11 28.5-11t27.5 11q12 11 11.5 27.5T282-226l-42 43q-11 12-28 11.5T183-183Zm297-297Z',
  dark: 'M480-120q-151 0-255.5-104.5T120-480q0-138 90-239.5T440-838q13-2 23 3.5t16 14.5q6 9 6.5 21t-7.5 23q-17 26-25.5 55t-8.5 61q0 90 63 153t153 63q31 0 61.5-9t54.5-25q11-7 22.5-6.5T819-479q10 5 15.5 15t3.5 24q-14 138-117.5 229T480-120Zm0-80q88 0 158-48.5T740-375q-20 5-40 8t-40 3q-123 0-209.5-86.5T364-660q0-20 3-40t8-40q-78 32-126.5 102T200-480q0 116 82 198t198 82Zm-10-270Z'
};
const ICONS = {
  chevron: MS(PATH.expandMore),
  folder:  MS(PATH.folder),
  folderOpen: MS(PATH.folderOpen),
  file:    MS(PATH.file),
  theme: { warm: MS(PATH.warm), light: MS(PATH.light), dark: MS(PATH.dark) }
};

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
function firstFile(nodes){
  const files = nodes.filter(n=>n.type==="file");
  const pref = files.find(n=>/^(readme|index|home)\b/i.test(n.name));
  if(pref) return pref.relpath;
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
      a.target="_blank"; a.rel="noopener noreferrer";
    }
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
function applyTransform(){
  page.style.transform = "scale("+state.zoom+")";
  canvas.style.width  = (page.offsetWidth  * state.zoom) + "px";
  canvas.style.height = (page.offsetHeight * state.zoom) + "px";
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
function toggleRail(id, key){
  const collapsed = document.getElementById(id).classList.toggle("collapsed");
  localStorage.setItem(key, collapsed ? "1" : "0");
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
    d.textContent="No files match “"+q+"”"; tree.appendChild(d); }
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
  const b=$("#b-font"); if(b) b.title="Reading font: "+f+" (click to switch)";
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
  setFont(localStorage.getItem("mdr-font") || "serif");
  updFull();
  if(localStorage.getItem("mdr-side")==="1") $("#sidebar").classList.add("collapsed");
  if(localStorage.getItem("mdr-out")==="1")  $("#outline").classList.add("collapsed");
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
try{ renderMathInElement(document.getElementById("page"),{delimiters:[
  {left:"$$",right:"$$",display:true},{left:"\\[",right:"\\]",display:true},
  {left:"$",right:"$",display:false},{left:"\\(",right:"\\)",display:false}],
  ignoredTags:["script","noscript","style","textarea","pre","code"],throwOnError:false}); }catch(e){}
</script>
</body>
</html>`
