# <img src="favicon.svg" alt="" width="28" align="absmiddle"> mdserve

A tiny, dependency-light Markdown docs server. Point it at a directory of `.md`
files and browse them as HTML with a live left-nav; or render them to a static
site.

## Features

A single-page Markdown reader: Markdown renders in the browser and the whole
front-end bundle is embedded, so it works **fully offline** — no CDN, no
network.

- **Reads the current directory** — a bare `mdserve` serves the folder you're
  standing in (free-port fallback if `127.0.0.1:8080` is taken, race-free via
  `net.Listen` on `:0`).
- **PDF-like reading view** — a centered page you can zoom (buttons, `Ctrl ±`,
  or `Ctrl`+wheel anchored at the cursor) and pan with the hand tool.
- **Three themes** — warm (default), light, dark; one button cycles them.
- **Folder tree + filter** — collapsible directories with Google Material
  Symbols file/folder icons; the `/` key jumps to the filter box.
- **Collapsible, resizable rails** — the file sidebar (`Cmd/Ctrl B`) and the
  outline (`` Cmd/Ctrl \ ``); widths and state persist in `localStorage`.
- **Auto outline** — a heading navigator with scroll-spy on the right.
- **In-page find** — `Cmd/Ctrl F`, highlight and step through matches.
- **Rich content** — GFM tables, syntax highlighting (highlight.js), Mermaid
  diagrams, and KaTeX math, all from the embedded bundle.
- **Remembers where you were** — per-doc scroll position is restored.
- **Live-reload** — the page polls for `.md` changes and re-renders in place.
- **Browser auto-open** (`--open`); a **request log** prints each doc view.
- **Static build** — `mdserve build` renders a self-contained, offline HTML
  tree (the vendor bundle is copied alongside).
- Path-traversal-safe; serves only `.md` under the configured root.

## Install

```sh
# prebuilt binary (Linux / macOS, amd64 or arm64) → ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/mostafa-re/mdserve/main/scripts/install.sh | sh

# or with Go
go install github.com/mostafa-re/mdserve@latest
```

Windows: grab the `.zip` for your arch from the
[releases page](https://github.com/mostafa-re/mdserve/releases). Set
`MDSERVE_VERSION` / `MDSERVE_BINDIR` to pin a version or change the install dir.

Update in place, or just check:

```sh
mdserve update          # download + replace the running binary with the latest release
mdserve update --check  # only report whether a newer release exists
mdserve version         # release tag, or dev+<commit> for a source build
```

Every `vX.Y.Z` tag is built for darwin/linux/windows × amd64/arm64 by GitHub
Actions and attached to the release (see `.github/workflows/release.yml`).

## Usage

```sh
mdserve                                         # serve the current dir at 127.0.0.1:8080
mdserve serve --dir docs --open                 # serve, free-port, open browser
mdserve serve --dir docs --addr :9000           # fixed port
mdserve build --dir docs --out site             # static HTML tree
mdserve serve --dir docs --default-doc plan/README.md   # custom landing doc
```

| flag | default | notes |
|---|---|---|
| `--dir` | `.` | directory of `.md` files (defaults to the current dir) |
| `--addr` | `127.0.0.1:8080` | loopback only; falls back to a free port if taken |
| `--default-doc` | `README.md` | doc opened at `/` |
| `--open` | off | open the browser |
| `--no-reload` | off | disable live-reload polling (serve) |
| `--no-cdn` | off | deprecated — assets are always embedded (no-op) |
| `--out` | — | output dir (build, required) |

## Design

One static Go binary. Markdown is rendered client-side by marked.js; the file
tree (`/api/tree`), file contents (`/raw`), and change polling (`/api/poll`)
are tiny JSON/text endpoints. The front-end vendor bundle — marked,
highlight.js, Mermaid, KaTeX (+ its fonts) — is embedded with `go:embed` and
served from `/vendor/`, so both the live reader and the static build work with
no network. Live-reload is an mtime poll, so there is zero extra dependency and
identical behavior across OSes. `gomarkdown` renders the static build only.

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
