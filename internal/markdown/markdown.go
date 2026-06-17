// Package markdown renders a GFM-flavored Markdown buffer to HTML. It is used by
// the static build (`mdserve build`); the live server renders Markdown in the
// browser with marked.js. The renderer is self-contained — standard library
// only — so mdserve has no third-party dependencies.
//
// Supported: ATX & setext headings (with auto IDs), paragraphs, GFM tables,
// fenced & indented code, ordered/unordered/nested/task lists, blockquotes,
// thematic breaks, links (inline + reference), images, autolinks (angle and
// bare URLs/emails), emphasis/strong/strikethrough, inline code, hard line
// breaks, backslash escapes, and pass-through of raw block & inline HTML. Math
// (`$…$`, `$$…$$`) is left as text for KaTeX to render in the browser, matching
// the live reader.
package markdown

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Render converts a Markdown buffer to HTML.
func Render(src []byte) []byte {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	// The renderer reserves NUL and SOH as internal sentinels (atom placeholders
	// and the hard-break marker); replace any in the input with U+FFFD so crafted
	// or binary input can't collide with them (CommonMark also maps NUL→U+FFFD).
	if strings.ContainsAny(text, "\x00\x01") {
		text = strings.NewReplacer("\x00", "�", "\x01", "�").Replace(text)
	}
	lines := strings.Split(text, "\n")
	r := &renderer{refs: map[string]linkRef{}, ids: map[string]int{}}
	lines = r.collectRefs(lines)
	r.blocks(lines)
	return []byte(r.b.String())
}

type renderer struct {
	b      strings.Builder
	refs   map[string]linkRef // link reference definitions, keyed by lowercased label
	ids    map[string]int     // heading slug -> times seen, for id de-duplication
	tight  bool               // inside a tight list: render item paragraphs without <p>
	inItem bool               // inside a list item: an ordered sublist may start at any number
}

type linkRef struct{ url, title string }

var (
	reATX       = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*$`)
	reSetext    = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	reFence     = regexp.MustCompile("^( {0,3})(`{3,}|~{3,})[ \t]*(\\S.*?)?[ \t]*$")
	reOLMarker  = regexp.MustCompile(`^( *)(\d{1,9})([.)])([ \t]+)(.*)$`)
	reULMarker  = regexp.MustCompile(`^( *)([-*+])([ \t]+)(.*)$`)
	reRefDef    = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:[ \t]*<?([^\s>]+)>?(?:[ \t]+(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'|\(((?:[^)\\]|\\.)*)\)))?[ \t]*$`)
	reTableSep  = regexp.MustCompile(`^ {0,3}\|?[ \t]*:?-+:?[ \t]*(\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$`)
	reEntity    = regexp.MustCompile(`^&(#[0-9]{1,8}|#[xX][0-9a-fA-F]{1,6}|[a-zA-Z][a-zA-Z0-9]{1,31});`)
	reStripImg  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reStripLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reStripRef  = regexp.MustCompile(`\[([^\]]*)\]\[[^\]]*\]`)
	reStripTag  = regexp.MustCompile(`<[^>]*>`)
	htmlBlockRe = regexp.MustCompile(`(?i)^ {0,3}(</?(?:address|article|aside|base|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h[1-6]|head|header|hr|html|iframe|legend|li|link|main|menu|nav|noframes|ol|optgroup|option|p|param|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)(?:[ \t/>]|$)|<!--|<\?|<![a-zA-Z]|<!\[CDATA\[)`)
)

// collectRefs strips link reference definitions ([id]: url "title") out of the
// line set, recording them for inline reference-link resolution.
func (r *renderer) collectRefs(lines []string) []string {
	out := lines[:0:0]
	for _, ln := range lines {
		if m := reRefDef.FindStringSubmatch(ln); m != nil {
			label := strings.ToLower(strings.TrimSpace(m[1]))
			title := unescapePunct(m[3] + m[4] + m[5])
			if _, dup := r.refs[label]; !dup {
				r.refs[label] = linkRef{url: m[2], title: title}
			}
			continue
		}
		out = append(out, ln)
	}
	return out
}

// blocks renders a sequence of block-level elements.
func (r *renderer) blocks(lines []string) {
	i := 0
	for i < len(lines) {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			i++
			continue
		}
		switch {
		case isHR(ln):
			r.b.WriteString("<hr>\n")
			i++
		case reATX.MatchString(ln):
			r.heading(ln)
			i++
		case isFence(ln):
			i = r.fencedCode(lines, i)
		case isBlockquote(ln):
			i = r.blockquote(lines, i)
		case isListStart(ln):
			i = r.list(lines, i)
		case isTableHeader(lines, i):
			i = r.table(lines, i)
		case htmlBlockRe.MatchString(ln):
			i = r.htmlBlock(lines, i)
		case isIndentedCode(ln):
			i = r.indentedCode(lines, i)
		default:
			i = r.paragraph(lines, i)
		}
	}
}

func (r *renderer) heading(ln string) {
	m := reATX.FindStringSubmatch(ln)
	id, clean := r.headingID(trimATXClosing(m[2]))
	r.writeHeading(len(m[1]), id, clean)
}

// writeHeading emits a heading; the id is attribute-escaped so an explicit
// {#id} can never break out of the attribute (XSS-safe).
func (r *renderer) writeHeading(level int, id, text string) {
	r.b.WriteString("<h")
	r.b.WriteByte(byte('0' + level))
	r.b.WriteString(` id="`)
	r.b.WriteString(escapeAttr(id))
	r.b.WriteString(`">`)
	r.b.WriteString(r.inline(text))
	r.b.WriteString("</h")
	r.b.WriteByte(byte('0' + level))
	r.b.WriteString(">\n")
}

// trimATXClosing removes a trailing ATX closing sequence (spaces then a run of
// #, e.g. "Foo ##" -> "Foo"), per CommonMark.
func trimATXClosing(s string) string {
	s = strings.TrimRight(s, " \t")
	i := len(s)
	for i > 0 && s[i-1] == '#' {
		i--
	}
	if i < len(s) && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
		return strings.TrimRight(s[:i], " \t")
	}
	return s
}

// headingID returns a de-duplicated slug for a heading, after stripping an
// optional trailing {#explicit-id}. The slug is derived from the heading's plain
// text (inline markup and HTML removed) so it matches the live reader's scheme.
func (r *renderer) headingID(text string) (id, clean string) {
	clean = text
	if i := strings.LastIndex(text, "{#"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i && strings.TrimSpace(text[j+1:]) == "" {
			id = strings.TrimSpace(text[i+2 : j])
			clean = strings.TrimSpace(text[:i])
		}
	}
	if id == "" {
		id = slugify(headingPlain(clean))
	}
	if n := r.ids[id]; n > 0 {
		r.ids[id] = n + 1
		id = id + "-" + strconv.Itoa(n)
	} else {
		r.ids[id] = 1
	}
	return id, clean
}

// headingPlain approximates a heading's rendered text content (what the reader
// slugs): inline images dropped, link/reference text kept, code/emphasis markers
// and raw HTML tags removed.
func headingPlain(s string) string {
	s = reStripImg.ReplaceAllString(s, "")
	s = reStripLink.ReplaceAllString(s, "$1")
	s = reStripRef.ReplaceAllString(s, "$1")
	s = reStripTag.ReplaceAllString(s, "")
	return strings.NewReplacer("`", "", "*", "", "_", "", "~", "").Replace(s)
}

// unescapePunct turns backslash-escaped ASCII punctuation (\" \' \) …) into the
// literal character, used for link/title text.
func unescapePunct(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// slugify mirrors the reader's Unicode-aware slug(): lowercase, drop all but
// letters, digits, underscore and hyphen (any script), collapse whitespace and
// hyphen runs to a single hyphen, trim hyphens. Empty results become "section".
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	for _, c := range s {
		switch {
		case unicode.IsLetter(c), unicode.IsDigit(c), c == '_', c == '-':
			sb.WriteRune(c)
		case unicode.IsSpace(c):
			sb.WriteByte('-')
		}
	}
	out := sb.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if out == "" {
		return "section"
	}
	return out
}

func (r *renderer) fencedCode(lines []string, i int) int {
	m := reFence.FindStringSubmatch(lines[i])
	indent := len(m[1])
	fence := m[2]
	info := strings.TrimSpace(m[3])
	lang := info
	if sp := strings.IndexAny(lang, " \t"); sp >= 0 {
		lang = lang[:sp]
	}
	var body strings.Builder
	i++
	for i < len(lines) {
		ln := lines[i]
		if c := reFence.FindStringSubmatch(ln); c != nil && c[2][0] == fence[0] && len(c[2]) >= len(fence) && strings.TrimSpace(c[3]) == "" {
			i++
			break
		}
		body.WriteString(trimIndent(ln, indent))
		body.WriteByte('\n')
		i++
	}
	r.b.WriteString("<pre><code")
	if lang != "" {
		r.b.WriteString(` class="language-`)
		r.b.WriteString(escapeAttr(lang))
		r.b.WriteString(`"`)
	}
	r.b.WriteString(">")
	r.b.WriteString(escapeCode(body.String()))
	r.b.WriteString("</code></pre>\n")
	return i
}

func (r *renderer) indentedCode(lines []string, i int) int {
	var body strings.Builder
	for i < len(lines) {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			// a blank line is part of the block only if more indented code follows
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) && isIndentedCode(lines[j]) {
				body.WriteByte('\n')
				i++
				continue
			}
			break
		}
		if !isIndentedCode(ln) {
			break
		}
		body.WriteString(trimIndent(ln, 4))
		body.WriteByte('\n')
		i++
	}
	r.b.WriteString("<pre><code>")
	r.b.WriteString(escapeCode(strings.TrimRight(body.String(), "\n") + "\n"))
	r.b.WriteString("</code></pre>\n")
	return i
}

func (r *renderer) blockquote(lines []string, i int) int {
	var inner []string
	for i < len(lines) {
		ln := lines[i]
		if isBlockquote(ln) {
			inner = append(inner, stripQuote(ln))
			i++
			continue
		}
		// lazy continuation: a non-blank, non-block line continues the quote
		if strings.TrimSpace(ln) != "" && !isBlankBlockStart(ln) {
			inner = append(inner, ln)
			i++
			continue
		}
		break
	}
	r.b.WriteString("<blockquote>\n")
	savedTight := r.tight
	r.tight = false // blockquote paragraphs are always wrapped
	r.blocks(inner)
	r.tight = savedTight
	r.b.WriteString("</blockquote>\n")
	return i
}

func (r *renderer) htmlBlock(lines []string, i int) int {
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
		r.b.WriteString(lines[i])
		r.b.WriteByte('\n')
		i++
	}
	return i
}

func (r *renderer) paragraph(lines []string, i int) int {
	var para []string
	for i < len(lines) {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			i++
			break
		}
		// setext heading: an underline under existing paragraph text
		if len(para) > 0 {
			if m := reSetext.FindStringSubmatch(ln); m != nil {
				level := 1
				if m[1][0] == '-' {
					level = 2
				}
				text := strings.TrimSpace(strings.Join(para, "\n"))
				id, clean := r.headingID(text)
				r.writeHeading(level, id, clean)
				return i + 1
			}
		}
		// a new block interrupts the paragraph
		if len(para) > 0 && r.paragraphInterrupt(lines, i) {
			break
		}
		// trim leading indent only — trailing spaces drive hard line breaks
		para = append(para, strings.TrimLeft(ln, " \t"))
		i++
	}
	if len(para) > 0 {
		if r.tight {
			r.b.WriteString(r.inlineLines(para))
			r.b.WriteString("\n")
		} else {
			r.b.WriteString("<p>")
			r.b.WriteString(r.inlineLines(para))
			r.b.WriteString("</p>\n")
		}
	}
	return i
}

// inlineLines renders paragraph lines, honoring hard line breaks (a line ending
// in two spaces or a backslash) as <br>.
func (r *renderer) inlineLines(para []string) string {
	for i, ln := range para {
		trimmed := strings.TrimRight(ln, " ")
		// a hard break needs a following line; a trailing space/backslash on the
		// last line is not a break (and never a dangling <br>).
		hard := i < len(para)-1 && (strings.HasSuffix(ln, "  ") || strings.HasSuffix(trimmed, `\`))
		if hard {
			para[i] = strings.TrimRight(trimmed, `\`) + "\x01" // sentinel for <br>
		} else {
			para[i] = trimmed
		}
	}
	joined := strings.Join(para, "\n")
	html := r.inline(joined)
	html = strings.ReplaceAll(html, "\x01\n", "<br>\n")
	html = strings.ReplaceAll(html, "\x01", "<br>\n")
	return html
}

// --- block predicates -----------------------------------------------------

func isFence(ln string) bool {
	m := reFence.FindStringSubmatch(ln)
	if m == nil {
		return false
	}
	// a backtick fence's info string may not contain a backtick (CommonMark)
	if m[2][0] == '`' && strings.Contains(m[3], "`") {
		return false
	}
	return true
}

func isBlockquote(ln string) bool {
	t := strings.TrimLeft(ln, " ")
	return strings.HasPrefix(t, ">") && leadingSpaces(ln) <= 3
}

func stripQuote(ln string) string {
	ln = strings.TrimLeft(ln, " ")
	ln = ln[1:] // drop '>'
	ln = strings.TrimPrefix(ln, " ")
	return ln
}

func isListStart(ln string) bool {
	if leadingSpaces(ln) > 3 {
		return false
	}
	return reULMarker.MatchString(ln) || reOLMarker.MatchString(ln)
}

func isIndentedCode(ln string) bool {
	return indentWidth(ln) >= 4 && strings.TrimSpace(ln) != ""
}

// paragraphInterrupt reports whether the line at i starts a block that breaks an
// open paragraph (everything except indented code, which is lazy text here).
func (r *renderer) paragraphInterrupt(lines []string, i int) bool {
	ln := lines[i]
	switch {
	case isHR(ln),
		reATX.MatchString(ln),
		isFence(ln),
		isBlockquote(ln),
		htmlBlockRe.MatchString(ln),
		isTableHeader(lines, i):
		return true
	case isListStart(ln):
		// a list that begins a non-empty item interrupts a paragraph; an ordered
		// list must start at 1 to do so (so "1999. prose" stays prose) — except
		// inside a list item, where an indented sublist may start at any number.
		if m := reOLMarker.FindStringSubmatch(ln); m != nil {
			return (r.inItem || m[2] == "1") && strings.TrimSpace(m[5]) != ""
		}
		if m := reULMarker.FindStringSubmatch(ln); m != nil {
			return strings.TrimSpace(m[4]) != ""
		}
	}
	return false
}

// isBlankBlockStart reports whether a line should end a lazy blockquote.
func isBlankBlockStart(ln string) bool {
	return isHR(ln) || reATX.MatchString(ln) || isFence(ln) || isListStart(ln) || htmlBlockRe.MatchString(ln)
}

// isHR reports a thematic break: 3+ of the same -, *, or _, spaces allowed
// between, indented at most 3 columns. (RE2 has no backreferences, so this is a
// hand-rolled check.)
func isHR(ln string) bool {
	if leadingSpaces(ln) > 3 {
		return false
	}
	t := strings.TrimSpace(ln)
	if len(t) < 3 {
		return false
	}
	ch := t[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case ch:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= 3
}

// --- shared helpers -------------------------------------------------------

func leadingSpaces(ln string) int {
	n := 0
	for n < len(ln) && ln[n] == ' ' {
		n++
	}
	return n
}

// indentWidth counts leading indentation in columns, treating a tab as advancing
// to the next multiple of four.
func indentWidth(ln string) int {
	w := 0
	for i := 0; i < len(ln); i++ {
		switch ln[i] {
		case ' ':
			w++
		case '\t':
			w += 4 - (w % 4)
		default:
			return w
		}
	}
	return w
}

// trimIndent removes up to n columns of leading whitespace.
func trimIndent(ln string, n int) string {
	i, w := 0, 0
	for i < len(ln) && w < n {
		switch ln[i] {
		case ' ':
			w++
			i++
		case '\t':
			w += 4 - (w % 4)
			i++
		default:
			return ln[i:]
		}
	}
	return ln[i:]
}

// escapeText HTML-escapes body text, preserving existing character entities.
func escapeText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			if m := reEntity.FindString(s[i:]); m != "" {
				b.WriteString(m)
				i += len(m) - 1
			} else {
				b.WriteString("&amp;")
			}
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// escapeCode escapes code spans/blocks: & < > only (quotes are fine in text).
func escapeCode(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// escapeAttr escapes a value destined for a double-quoted HTML attribute.
func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
