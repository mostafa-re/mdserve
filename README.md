# <img src="favicon.svg" alt="" width="28" align="absmiddle"> mdserve

A tiny, dependency-light Markdown docs server. Point it at a directory of `.md`
files and browse them as HTML with a live left-nav; or render them to a static
site.

## Features

- **Free-port fallback** — if `127.0.0.1:8080` is taken, grabs an OS-assigned free port
  (`net.Listen` on `:0`, race-free) and prints the URL.
- **Browser auto-open** (`--open`) — `open` / `xdg-open` / `rundll32`,
  no-op when headless / over SSH / in CI.
- **Live-reload on save** — the page reloads when any `.md` changes (mtime poll
  over SSE; no extra dependency, same on every OS).
- **Folder-tree nav** — collapsible directories with SVG file/folder icons
  (open/closed), the branch to the current doc auto-expanded, plus a filter box.
- **Collapsible + resizable sidebar** — toggle it shut, or drag its edge to
  resize; the width and collapsed state persist in `localStorage`.
- **Theme toggle** — one button cycles dark → light → warm (defaults to dark),
  plus zoom in/out/reset and a back-to-top button.
- **In-doc search** — find-and-highlight within the current doc (Enter / Shift+
  Enter to step, with a match counter), separate from the file filter.
- **Remembers where you were** — per-doc scroll position is restored, so paging
  between docs doesn't jump you back to the top.
- **Request log** — each doc view prints `method path → status (latency)`.
- **Polish** — tables and optional syntax highlighting + Mermaid diagrams via
  CDN (`--no-cdn` for an offline-pure page).
- **Static build** — `mdserve build` renders a self-contained HTML tree.
- Path-traversal-safe; serves only `.md` under the configured root.

## Install

```sh
go install github.com/mostafa-re/mdserve@latest
# or, per-project pinned (Go 1.24+ tool directive):
go get -tool github.com/mostafa-re/mdserve@v0.1.0
# or a prebuilt binary / Homebrew tap (see .goreleaser.yaml)
```

## Usage

```sh
mdserve serve --dir docs --open                 # serve, free-port, open browser
mdserve serve --dir docs --addr :9000 --no-cdn  # fixed port, offline-pure
mdserve build --dir docs --out site             # static HTML tree
mdserve serve --dir docs --default-doc plan/README.md   # custom landing doc
```

| flag | default | notes |
|---|---|---|
| `--dir` | `docs` | directory of `.md` files |
| `--addr` | `127.0.0.1:8080` | loopback only; falls back to a free port if taken |
| `--default-doc` | `README.md` | doc opened at `/` |
| `--open` | off | open the browser |
| `--no-cdn` | off | omit Mermaid/highlight.js CDN assets |
| `--no-reload` | off | disable live-reload (serve) |
| `--out` | — | output dir (build, required) |

## Design

One static Go binary, one Markdown library (`gomarkdown`), stdlib for
everything else — no JS framework, no theme engine, no fsnotify. Live-reload is
an mtime poll + Server-Sent Events so there is zero extra dependency and
identical behavior across OSes. The doc tree is read on each request so the page
always reflects the last save. CDN assets (highlight.js, Mermaid) are the only
network dependency and are fully opt-out via `--no-cdn`.

## Use in a project

A Makefile target that shells out to the installed binary:

```make
doc:        ## Serve docs/ as HTML (free port, opens browser)
	@mdserve serve --dir docs --default-doc README.md --open
doc-build:  ## Render docs/ to docs/_site/
	@mdserve build --dir docs --out docs/_site
```

> **Local docs only.** mdserve has no auth and renders raw repo Markdown; it
> binds for local browsing. Never run it as a public/deployed service.

## License

[MIT](LICENSE).
