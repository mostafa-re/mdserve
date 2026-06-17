package markdown

import (
	"strings"
	"testing"
)

func render(s string) string { return string(Render([]byte(s))) }

// contains asserts every wanted substring is present in the rendered HTML.
func contains(t *testing.T, src string, want ...string) {
	t.Helper()
	got := render(src)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("render(%q) missing %q\n--- got ---\n%s", src, w, got)
		}
	}
}

// absent asserts none of the unwanted substrings are present.
func absent(t *testing.T, src string, bad ...string) {
	t.Helper()
	got := render(src)
	for _, b := range bad {
		if strings.Contains(got, b) {
			t.Errorf("render(%q) unexpectedly contains %q\n--- got ---\n%s", src, b, got)
		}
	}
}

func TestHeadings(t *testing.T) {
	contains(t, "# Home", `<h1 id="home">Home</h1>`)
	contains(t, "### A B C", `<h3 id="a-b-c">A B C</h3>`)
	contains(t, "## Hello, World!", `<h2 id="hello-world">Hello, World!</h2>`)
	// setext
	contains(t, "Title\n=====", `<h1 id="title">Title</h1>`)
	contains(t, "Sub\n---", `<h2 id="sub">Sub</h2>`)
	// explicit id
	contains(t, "# Custom {#myid}", `id="myid"`, ">Custom</h1>")
	// duplicate ids de-duplicate
	contains(t, "# Dup\n\n# Dup", `id="dup"`, `id="dup-1"`)
}

func TestEmphasis(t *testing.T) {
	contains(t, "**bold**", "<strong>bold</strong>")
	contains(t, "*italic*", "<em>italic</em>")
	contains(t, "_italic_", "<em>italic</em>")
	contains(t, "__bold__", "<strong>bold</strong>")
	contains(t, "***both***", "<em><strong>both</strong></em>")
	contains(t, "a **b _c_ d** e", "<strong>b <em>c</em> d</strong>")
	// NoIntraEmphasis: underscores inside a word are literal
	absent(t, "foo_bar_baz", "<em>")
	// strikethrough
	contains(t, "~~gone~~", "<del>gone</del>")
}

func TestInlineCode(t *testing.T) {
	contains(t, "use `x < y` here", "<code>x &lt; y</code>")
	contains(t, "``a `b` c``", "<code>a `b` c</code>")
	// no emphasis inside code
	contains(t, "`a*b*c`", "<code>a*b*c</code>")
}

func TestLinksAndImages(t *testing.T) {
	contains(t, "[t](http://x.com)", `<a href="http://x.com"`, `target="_blank"`, ">t</a>")
	contains(t, `[t](http://x.com "ti")`, `title="ti"`)
	contains(t, "![alt](/img.png)", `<img src="/img.png" alt="alt">`)
	// reference link
	contains(t, "[t][ref]\n\n[ref]: http://y.com", `href="http://y.com"`)
	// emphasis around a link
	contains(t, "**[t](u)**", "<strong><a ")
}

func TestAutolinks(t *testing.T) {
	contains(t, "<http://x.com>", `<a href="http://x.com"`)
	contains(t, "see http://x.com here", `<a href="http://x.com"`)
	contains(t, "(see https://x.com/a).", `href="https://x.com/a"`, ").")
	contains(t, "<a@b.com>", `href="mailto:a@b.com"`)
}

func TestCodeFence(t *testing.T) {
	contains(t, "```go\nfmt.Println()\n```", `<pre><code class="language-go">`, "fmt.Println()")
	contains(t, "```mermaid\nflowchart LR\n```", `class="language-mermaid"`)
	// html inside fence is escaped, not interpreted
	contains(t, "```\n<div>\n```", "&lt;div&gt;")
	absent(t, "```\n**x**\n```", "<strong>")
}

func TestLists(t *testing.T) {
	contains(t, "- a\n- b", "<ul>", "<li>a</li>", "<li>b</li>", "</ul>")
	contains(t, "1. a\n2. b", "<ol>", "<li>a</li>", "<li>b</li>", "</ol>")
	contains(t, "3. a\n4. b", `<ol start="3">`)
	// nested
	contains(t, "- a\n  - b", "<li>a", "<ul>", "<li>b</li>")
	// task list
	contains(t, "- [ ] todo\n- [x] done", `<input type="checkbox" disabled>`, `checked disabled>`)
	// loose list wraps items in <p>
	contains(t, "- a\n\n- b", "<li>\n<p>a</p>")
}

func TestTable(t *testing.T) {
	src := "| a | b |\n|---|:-:|\n| 1 | 2 |"
	contains(t, src, "<table>", "<thead>", "<th>a</th>", `<th style="text-align:center">b</th>`, "<tbody>", "<td>1</td>")
}

func TestBlockquote(t *testing.T) {
	contains(t, "> quoted", "<blockquote>", "<p>quoted</p>", "</blockquote>")
	contains(t, "> a\n> b", "<p>a\nb</p>")
}

func TestThematicBreak(t *testing.T) {
	contains(t, "a\n\n---\n\nb", "<hr>")
	contains(t, "***", "<hr>")
}

func TestRawHTMLPassthrough(t *testing.T) {
	contains(t, "<details>\n<summary>x</summary>\n</details>", "<details>", "<summary>x</summary>")
	contains(t, "text <br> more", "<br>")
	contains(t, `# <img src="i.svg"> Title`, `<img src="i.svg">`)
}

func TestEntitiesAndMath(t *testing.T) {
	// existing entities are preserved, bare & is escaped
	contains(t, "A &amp; B & C", "A &amp; B &amp; C")
	// math delimiters survive for client-side KaTeX
	contains(t, "inline $e^{i\\pi}+1=0$ math", "$e^{i\\pi}+1=0$")
	contains(t, "price $100 stays text", "$100")
}

func TestEscapesAndBreaks(t *testing.T) {
	contains(t, `\*not emphasis\*`, "*not emphasis*")
	absent(t, `\*not emphasis\*`, "<em>")
	// hard break: two trailing spaces
	contains(t, "line one  \nline two", "<br>")
}

// XSS-ish: angle brackets in plain text must be escaped, not emitted as tags.
func TestPlainAnglesEscaped(t *testing.T) {
	contains(t, "1 < 2 and 3 > 2", "1 &lt; 2 and 3 &gt; 2")
}

// --- regressions found by adversarial differential testing ----------------

// An explicit {#id} must be attribute-escaped so a crafted heading can't break
// out of id="..." and inject a handler or a <script> tag.
func TestHeadingExplicitIDEscaped(t *testing.T) {
	absent(t, `# T {#a" onclick="alert(1)}`, `onclick="alert(1)">`)
	contains(t, `# T {#a" onclick="alert(1)}`, `&quot;`)
	absent(t, `# X {#"><script>alert(1)</script>}`, "<script>alert(1)</script>")
}

func TestATXClosingSequenceStripped(t *testing.T) {
	contains(t, "# Foo #", `<h1 id="foo">Foo</h1>`)
	contains(t, "## Bar ##", `<h2 id="bar">Bar</h2>`)
	contains(t, "# C#", `>C#</h1>`) // a trailing # not preceded by space is kept
}

func TestNonASCIISlug(t *testing.T) {
	contains(t, "# 你好世界", `id="你好世界"`)
	contains(t, "# مرحبا بالعالم", `id="مرحبا-بالعالم"`)
	contains(t, "# Hello !!!", `id="hello"`) // no dangling trailing hyphen
}

// heading id derives from text content, not raw markdown (matches the reader).
func TestHeadingIDFromText(t *testing.T) {
	contains(t, `# <img src="i.svg"> Title`, `id="title"`)
	contains(t, "# A `code` and [link](u)", `id="a-code-and-link"`)
}

func TestEmphasisNesting(t *testing.T) {
	contains(t, "*x**y**z*", "<em>x<strong>y</strong>z</em>")
	contains(t, "**a*b*c**", "<strong>a<em>b</em>c</strong>")
	contains(t, "**foo*bar**", "<strong>foo*bar</strong>")
	// interior intraword underscore stays literal, not dropped
	contains(t, "_x_y_", "<em>x_y</em>")
	absent(t, "*x**y*", "<em></em>") // no empty emphasis tags
}

func TestWWWAndBareURLEmphasis(t *testing.T) {
	contains(t, "www.example.com", `href="http://www.example.com"`, ">www.example.com</a>")
	contains(t, "see *http://example.com* end", "<em><a ", `href="http://example.com"`)
	absent(t, "see *http://example.com* end", "example.com*") // the * is not swallowed into the URL
}

func TestEscapedQuoteTitle(t *testing.T) {
	contains(t, `![logo](a.png "say \"hi\"")`, `<img src="a.png"`, `title="say &quot;hi&quot;"`)
	contains(t, `[t](http://x.com "ti\"tle")`, `title="ti&quot;tle"`, ">t</a>")
}

func TestListLazyContinuation(t *testing.T) {
	contains(t, "- a\nb", "<li>a\nb</li>")
	contains(t, "5. five\nlazy\n6. six", `<ol start="5">`, "<li>five\nlazy</li>", "<li>six</li>")
}

// an indented ordered sublist may start at any number inside a list item.
func TestNestedOrderedSublist(t *testing.T) {
	contains(t, "- a\n  5. five\n  6. six", "<li>a", `<ol start="5">`, "<li>five</li>", "<li>six</li>")
	// but a non-1 ordered list must NOT turn flowing prose into a list
	absent(t, "The year was\n1999. It was great.", "<ol")
}

func TestTablePipeInCodeSpan(t *testing.T) {
	contains(t, "| a | b |\n| - | - |\n| `x|y` | z |", "<code>x|y</code>", "<td>z</td>")
}

// control bytes used as internal sentinels must not crash or corrupt output.
func TestControlBytesSafe(t *testing.T) {
	_ = render("a\x00b") // must not panic
	_ = render("a\x01b")
	absent(t, "a\x01b", "<br>")
}

// a hard-break trigger on the last line must not leave a dangling <br>.
func TestNoDanglingBreak(t *testing.T) {
	absent(t, "only line  ", "<br>")
	absent(t, `only line\`, "<br>")
}

func TestFenceInfoStringNoBacktick(t *testing.T) {
	// a backtick fence whose info string has a backtick is a paragraph, not code
	contains(t, "``` aa ```\nfoo", "<p>", "<code>aa</code>")
	absent(t, "``` aa ```\nfoo", "language-")
}
