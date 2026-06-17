// Package markdown renders a GFM-flavored Markdown buffer to HTML. It is used by
// the static build (`mdserve build`); the live server renders Markdown in the
// browser with marked.js.
package markdown

import (
	"github.com/gomarkdown/markdown"
	gohtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// Render converts a Markdown buffer to HTML (tables, fenced code, autolinks,
// heading IDs). Used by the static build only — the live server renders in the
// browser.
func Render(src []byte) []byte {
	exts := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(exts)
	renderer := gohtml.NewRenderer(gohtml.RendererOptions{Flags: gohtml.CommonFlags | gohtml.HrefTargetBlank})
	return markdown.Render(p.Parse(src), renderer)
}
