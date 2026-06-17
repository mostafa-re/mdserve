package markdown

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reAutoURL   = regexp.MustCompile(`^https?://[^\s<>]+`)
	reBareURL   = regexp.MustCompile(`^(https?://|www\.)[^\s<>]+`)
	reEmail     = regexp.MustCompile(`^[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+`)
	reAngleMail = regexp.MustCompile(`^<([A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+)>`)
	reHTMLTag   = regexp.MustCompile(`(?is)^<(?:/[a-zA-Z][a-zA-Z0-9-]*\s*>|[a-zA-Z][a-zA-Z0-9-]*(?:\s+[^<>]*?)?/?>|!--.*?-->)`)
)

// inline renders inline Markdown to HTML. It works in two phases: extract spans
// that emphasis must not touch ("atoms" — code, links, images, autolinks, raw
// HTML, escapes) into placeholders, run emphasis/strikethrough over the residue,
// then splice the atoms back in.
func (r *renderer) inline(s string) string {
	var atoms []string
	var sb strings.Builder
	emit := func(html string) {
		sb.WriteString("\x00")
		sb.WriteString(strconv.Itoa(len(atoms)))
		sb.WriteString("\x00")
		atoms = append(atoms, html)
	}

	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && isASCIIPunct(s[i+1]):
			emit(escapeText(string(s[i+1])))
			i += 2
		case c == '`':
			if html, ni, ok := codeSpan(s, i); ok {
				emit(html)
				i = ni
			} else {
				sb.WriteByte(c)
				i++
			}
		case c == '!' && i+1 < len(s) && s[i+1] == '[':
			if html, ni, ok := r.linkOrImage(s, i, true); ok {
				emit(html)
				i = ni
			} else {
				sb.WriteByte(c)
				i++
			}
		case c == '[':
			if html, ni, ok := r.linkOrImage(s, i, false); ok {
				emit(html)
				i = ni
			} else {
				sb.WriteByte(c)
				i++
			}
		case c == '<':
			if html, ni, ok := angleSpan(s, i); ok {
				emit(html)
				i = ni
			} else {
				sb.WriteByte(c) // escaped later by emphasize()
				i++
			}
		case (c == 'h' || c == 'w') && isURLBoundary(s, i):
			if m := reBareURL.FindString(s[i:]); m != "" {
				url, _ := trimURLTrailing(m)
				href := url
				if strings.HasPrefix(href, "www.") {
					href = "http://" + href // www. autolinks need a scheme or they resolve relative
				}
				emit(autolinkHTML(href, url, false))
				i += len(url) // trailing punctuation stays in the stream (e.g. a closing * is an emphasis delimiter)
			} else {
				sb.WriteByte(c)
				i++
			}
		case isEmailStart(s, i):
			if m := reEmail.FindString(s[i:]); m != "" {
				emit(autolinkHTML("mailto:"+m, m, false))
				i += len(m)
			} else {
				sb.WriteByte(c)
				i++
			}
		default:
			sb.WriteByte(c)
			i++
		}
	}

	out := emphasize(sb.String())
	// splice atoms back in
	var res strings.Builder
	for j := 0; j < len(out); j++ {
		if out[j] == '\x00' {
			k := j + 1
			for k < len(out) && out[k] != '\x00' {
				k++
			}
			idx, _ := strconv.Atoi(out[j+1 : k])
			if idx >= 0 && idx < len(atoms) {
				res.WriteString(atoms[idx])
			}
			j = k
			continue
		}
		res.WriteByte(out[j])
	}
	return res.String()
}

// codeSpan parses a backtick code span starting at i. Per CommonMark, the
// closing run must match the opening run length, and one space is stripped from
// each side when the content is space-padded but not all spaces.
func codeSpan(s string, i int) (string, int, bool) {
	n := runLen(s, i, '`')
	j := i + n
	for j < len(s) {
		if s[j] == '`' {
			m := runLen(s, j, '`')
			if m == n {
				content := s[i+n : j]
				content = strings.ReplaceAll(content, "\n", " ")
				if len(content) >= 2 && content[0] == ' ' && content[len(content)-1] == ' ' && strings.TrimSpace(content) != "" {
					content = content[1 : len(content)-1]
				}
				return "<code>" + escapeCode(content) + "</code>", j + m, true
			}
			j += m
			continue
		}
		j++
	}
	return "", 0, false
}

// linkOrImage parses an inline or reference link/image starting at the '[' (or
// '![' when image). Supports [text](url "title"), [text][ref], [text][] and
// shortcut [ref].
func (r *renderer) linkOrImage(s string, i int, image bool) (string, int, bool) {
	if image {
		i++ // skip '!'
	}
	// match the bracketed label, honoring one level of nested brackets
	textEnd, depth := -1, 0
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '[':
			depth++
		case ']':
			if depth == 0 {
				textEnd = j
			} else {
				depth--
			}
		}
		if textEnd >= 0 {
			break
		}
	}
	if textEnd < 0 {
		return "", 0, false
	}
	label := s[i+1 : textEnd]
	rest := textEnd + 1

	// inline form: (url "title")
	if rest < len(s) && s[rest] == '(' {
		if url, title, ni, ok := parseInlineDest(s, rest); ok {
			return r.renderLink(label, url, title, image), ni, true
		}
	}
	// reference forms
	var refKey string
	ni := rest
	if rest < len(s) && s[rest] == '[' {
		// [text][ref] or [text][]
		end := strings.IndexByte(s[rest+1:], ']')
		if end < 0 {
			return "", 0, false
		}
		ref := s[rest+1 : rest+1+end]
		if strings.TrimSpace(ref) == "" {
			refKey = label
		} else {
			refKey = ref
		}
		ni = rest + 1 + end + 1
	} else {
		// shortcut [ref]
		refKey = label
	}
	if def, ok := r.refs[strings.ToLower(strings.TrimSpace(refKey))]; ok {
		return r.renderLink(label, def.url, def.title, image), ni, true
	}
	return "", 0, false
}

// parseInlineDest parses "(url "title")" starting at the '('.
func parseInlineDest(s string, i int) (url, title string, next int, ok bool) {
	j := i + 1
	for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n') {
		j++
	}
	// destination: <...> or bare up to space or ')'
	if j < len(s) && s[j] == '<' {
		end := strings.IndexByte(s[j:], '>')
		if end < 0 {
			return "", "", 0, false
		}
		url = s[j+1 : j+end]
		j += end + 1
	} else {
		depth := 0
		st := j
		for j < len(s) {
			c := s[j]
			if c == '\\' && j+1 < len(s) {
				j += 2
				continue
			}
			if c == '(' {
				depth++
			} else if c == ')' {
				if depth == 0 {
					break
				}
				depth--
			} else if c == ' ' || c == '\t' || c == '\n' {
				break
			}
			j++
		}
		url = s[st:j]
	}
	for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n') {
		j++
	}
	// optional title in " ", ' ', or ( ), honoring backslash escapes inside it
	if j < len(s) && (s[j] == '"' || s[j] == '\'' || s[j] == '(') {
		open := s[j]
		close := open
		if open == '(' {
			close = ')'
		}
		var tb strings.Builder
		k := j + 1
		for k < len(s) {
			if s[k] == '\\' && k+1 < len(s) && isASCIIPunct(s[k+1]) {
				tb.WriteByte(s[k+1])
				k += 2
				continue
			}
			if s[k] == close {
				break
			}
			tb.WriteByte(s[k])
			k++
		}
		if k < len(s) && s[k] == close {
			title = tb.String()
			j = k + 1
		}
	}
	for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n') {
		j++
	}
	if j < len(s) && s[j] == ')' {
		return strings.TrimSpace(url), title, j + 1, true
	}
	return "", "", 0, false
}

func (r *renderer) renderLink(label, url, title string, image bool) string {
	if image {
		var b strings.Builder
		b.WriteString(`<img src="`)
		b.WriteString(escapeAttr(url))
		b.WriteString(`" alt="`)
		b.WriteString(escapeAttr(stripInline(label)))
		b.WriteString(`"`)
		if title != "" {
			b.WriteString(` title="`)
			b.WriteString(escapeAttr(title))
			b.WriteString(`"`)
		}
		b.WriteString(">")
		return b.String()
	}
	var b strings.Builder
	b.WriteString(`<a href="`)
	b.WriteString(escapeAttr(url))
	b.WriteString(`"`)
	if title != "" {
		b.WriteString(` title="`)
		b.WriteString(escapeAttr(title))
		b.WriteString(`"`)
	}
	// open links in a new tab — matches the previous renderer's HrefTargetBlank
	b.WriteString(` target="_blank" rel="noopener">`)
	b.WriteString(r.inline(label))
	b.WriteString("</a>")
	return b.String()
}

// angleSpan handles <autolink>, <mailto:…>, <email>, and raw inline HTML tags.
func angleSpan(s string, i int) (string, int, bool) {
	if m := reAngleMail.FindStringSubmatch(s[i:]); m != nil {
		return autolinkHTML("mailto:"+m[1], m[1], false), i + len(m[0]), true
	}
	if strings.HasPrefix(s[i:], "<") {
		if end := strings.IndexByte(s[i:], '>'); end > 0 {
			inner := s[i+1 : i+end]
			if reAutoURL.MatchString(inner) {
				return autolinkHTML(inner, inner, false), i + end + 1, true
			}
		}
	}
	if m := reHTMLTag.FindString(s[i:]); m != "" {
		return m, i + len(m), true // raw HTML, passed through verbatim
	}
	return "", 0, false
}

func autolinkHTML(href, text string, _ bool) string {
	return `<a href="` + escapeAttr(href) + `" target="_blank" rel="noopener">` + escapeText(text) + `</a>`
}

// --- emphasis -------------------------------------------------------------

// emphasize processes *, _ (emphasis/strong) and ~~ (strikethrough) over text
// that still contains atom placeholders, escaping the remaining literal text.
func emphasize(s string) string {
	// A node is literal text, an opaque placeholder atom, or a delimiter run.
	type node struct {
		text     string // literal text (unescaped) or placeholder atom
		ph       bool   // placeholder atom — emitted verbatim
		isDelim  bool
		ch       byte
		n        int // remaining unconsumed delimiter chars (rendered literally)
		canOpen  bool
		canClose bool
		active   bool   // still matchable on the delimiter stack
		pre      string // closing tags, emitted before the literal remainder
		post     string // opening tags, emitted after the literal remainder
	}
	var nodes []*node
	prev := byte(' ')
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\x00' { // placeholder atom: \x00<index>\x00
			k := i + 1
			for k < len(s) && s[k] != '\x00' {
				k++
			}
			nodes = append(nodes, &node{text: s[i : k+1], ph: true})
			prev = 'a' // an atom counts as a word char for flanking
			i = k + 1
			continue
		}
		if c == '*' || c == '_' || c == '~' {
			n := runLen(s, i, c)
			if c == '~' && n != 2 { // only ~~ is strikethrough; other runs are literal
				nodes = append(nodes, &node{text: s[i : i+n]})
				prev = c
				i += n
				continue
			}
			next := byte(' ')
			if i+n < len(s) {
				next = s[i+n]
			}
			bWS, aWS := isFlankWS(prev), isFlankWS(next)
			bP, aP := isASCIIPunct(prev), isASCIIPunct(next)
			leftFlank := !aWS && (!aP || bWS || bP)
			rightFlank := !bWS && (!bP || aWS || aP)
			nd := &node{isDelim: true, ch: c, n: n, active: true}
			if c == '_' {
				nd.canOpen = leftFlank && (!rightFlank || bP)
				nd.canClose = rightFlank && (!leftFlank || aP)
			} else {
				nd.canOpen = leftFlank
				nd.canClose = rightFlank
			}
			nodes = append(nodes, nd)
			prev = c
			i += n
			continue
		}
		j := i
		for j < len(s) && s[j] != '*' && s[j] != '_' && s[j] != '~' && s[j] != '\x00' {
			j++
		}
		nodes = append(nodes, &node{text: s[i:j]})
		prev = s[j-1]
		i = j
	}

	// Delimiter matching — a trimmed-down CommonMark process_emphasis: for each
	// closer (left to right) find the nearest active opener of the same char,
	// honoring the "rule of 3", wrap the span, and deactivate the delimiters in
	// between (their leftover characters still render literally, so nothing is
	// lost).
	var dl []int
	for idx, nd := range nodes {
		if nd.isDelim {
			dl = append(dl, idx)
		}
	}
	tags := func(ch byte, strong bool) (string, string) {
		switch {
		case ch == '~':
			return "<del>", "</del>"
		case strong:
			return "<strong>", "</strong>"
		default:
			return "<em>", "</em>"
		}
	}
	ci := 0
	for ci < len(dl) {
		c := nodes[dl[ci]]
		if !c.active || !c.canClose {
			ci++
			continue
		}
		found, oi := false, ci-1
		for ; oi >= 0; oi-- {
			o := nodes[dl[oi]]
			if !o.active || !o.canOpen || o.ch != c.ch {
				continue
			}
			odd := (c.canOpen || o.canClose) && (o.n+c.n)%3 == 0 && !(o.n%3 == 0 && c.n%3 == 0)
			if !odd {
				found = true
				break
			}
		}
		if !found {
			if !c.canOpen {
				c.active = false
			}
			ci++
			continue
		}
		o := nodes[dl[oi]]
		use := 1
		if o.n >= 2 && c.n >= 2 {
			use = 2
		}
		open, clz := tags(c.ch, use == 2)
		o.post = open + o.post // opener: tag goes after its (left-side) leftover
		c.pre = c.pre + clz    // closer: tag goes before its (right-side) leftover
		o.n -= use
		c.n -= use
		for k := oi + 1; k < ci; k++ {
			nodes[dl[k]].active = false
		}
		if o.n == 0 {
			o.active = false
		}
		if c.n == 0 {
			c.active = false
			ci++
		}
	}

	var b strings.Builder
	for _, nd := range nodes {
		b.WriteString(nd.pre)
		switch {
		case nd.ph:
			b.WriteString(nd.text)
		case nd.isDelim:
			if nd.n > 0 {
				b.WriteString(strings.Repeat(string(nd.ch), nd.n))
			}
		default:
			b.WriteString(escapeText(nd.text))
		}
		b.WriteString(nd.post)
	}
	return b.String()
}

// --- tiny scanning helpers ------------------------------------------------

func runLen(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

func isASCIIPunct(c byte) bool {
	return (c >= '!' && c <= '/') || (c >= ':' && c <= '@') || (c >= '[' && c <= '`') || (c >= '{' && c <= '~')
}

func isFlankWS(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

func isURLBoundary(s string, i int) bool {
	if i > 0 {
		p := s[i-1]
		if p == '/' || p == '@' || (p >= 'a' && p <= 'z') || (p >= 'A' && p <= 'Z') || (p >= '0' && p <= '9') {
			return false
		}
	}
	return reBareURL.MatchString(s[i:])
}

func isEmailStart(s string, i int) bool {
	c := s[i]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
		return false
	}
	if i > 0 {
		p := s[i-1]
		if (p >= 'a' && p <= 'z') || (p >= 'A' && p <= 'Z') || (p >= '0' && p <= '9') || p == '@' || p == '.' {
			return false
		}
	}
	return reEmail.MatchString(s[i:])
}

// trimURLTrailing splits trailing punctuation off a bare URL (GFM: a trailing
// ., ,, ;, :, !, ? and unbalanced ) are not part of the link).
func trimURLTrailing(u string) (url, trail string) {
	for len(u) > 0 {
		c := u[len(u)-1]
		if strings.IndexByte(".,;:!?*_~", c) >= 0 {
			trail = string(c) + trail
			u = u[:len(u)-1]
			continue
		}
		if c == ')' && strings.Count(u, ")") > strings.Count(u, "(") {
			trail = string(c) + trail
			u = u[:len(u)-1]
			continue
		}
		break
	}
	return u, trail
}

// stripInline removes inline markup characters for use in an image alt
// attribute (a plain-text approximation).
func stripInline(s string) string {
	repl := strings.NewReplacer("*", "", "_", "", "`", "", "~", "")
	return repl.Replace(s)
}
