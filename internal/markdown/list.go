package markdown

import (
	"strconv"
	"strings"
)

type listItem struct {
	lines         []string
	contentIndent int
}

// list parses a bullet or ordered list starting at lines[start], including
// nested lists and task-list checkboxes, and returns the index after it.
func (r *renderer) list(lines []string, start int) int {
	ordered := reOLMarker.MatchString(lines[start]) && leadingSpaces(lines[start]) <= 3
	startNum := 1
	if ordered {
		m := reOLMarker.FindStringSubmatch(lines[start])
		startNum, _ = strconv.Atoi(m[2])
	}
	base := leadingSpaces(lines[start])

	var items []*listItem
	loose := false
	pendingBlank := false
	i := start
	for i < len(lines) {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" {
			pendingBlank = true
			i++
			continue
		}
		ind := leadingSpaces(ln)
		if ind == base && isSameKind(ln, ordered) {
			if len(items) > 0 && pendingBlank {
				loose = true
			}
			content, ci := stripMarker(ln)
			it := &listItem{contentIndent: ci}
			if content != "" {
				it.lines = append(it.lines, content)
			}
			items = append(items, it)
			pendingBlank = false
			i++
			continue
		}
		if cur := lastItem(items); cur != nil && ind >= cur.contentIndent {
			if pendingBlank {
				loose = true
				cur.lines = append(cur.lines, "")
				pendingBlank = false
			}
			cur.lines = append(cur.lines, deindent(ln, cur.contentIndent))
			i++
			continue
		}
		if cur := lastItem(items); cur != nil && !pendingBlank && !startsNewBlock(lines, i) {
			// lazy paragraph continuation — a wrapped line (even at column 0) that
			// isn't a new block keeps flowing the current item's paragraph
			cur.lines = append(cur.lines, strings.TrimSpace(ln))
			i++
			continue
		}
		break
	}

	tag := "ul"
	if ordered {
		tag = "ol"
	}
	r.b.WriteString("<")
	r.b.WriteString(tag)
	if ordered && startNum != 1 {
		r.b.WriteString(` start="`)
		r.b.WriteString(strconv.Itoa(startNum))
		r.b.WriteString(`"`)
	}
	r.b.WriteString(">\n")

	savedTight := r.tight
	r.tight = !loose
	for _, it := range items {
		check := taskCheckbox(it)
		body := r.renderItem(trimTrailingBlank(it.lines))
		if loose {
			r.b.WriteString("<li>\n")
			r.b.WriteString(check)
			r.b.WriteString(body)
			r.b.WriteString("\n</li>\n")
		} else {
			r.b.WriteString("<li>")
			r.b.WriteString(check)
			r.b.WriteString(body)
			r.b.WriteString("</li>\n")
		}
	}
	r.tight = savedTight

	r.b.WriteString("</")
	r.b.WriteString(tag)
	r.b.WriteString(">\n")
	return i
}

// renderItem renders one list item's content to HTML, sharing the document's
// link refs, heading IDs, and tightness, and trims the trailing newline so a
// single-paragraph tight item reads as <li>text</li>.
func (r *renderer) renderItem(lines []string) string {
	sub := &renderer{refs: r.refs, ids: r.ids, tight: r.tight, inItem: true}
	sub.blocks(lines)
	return strings.TrimRight(sub.b.String(), "\n")
}

// startsNewBlock reports whether the line at i begins a block, so a list's lazy
// paragraph continuation must stop rather than absorb it.
func startsNewBlock(lines []string, i int) bool {
	ln := lines[i]
	return isHR(ln) || reATX.MatchString(ln) || isFence(ln) || isBlockquote(ln) ||
		isListStart(ln) || htmlBlockRe.MatchString(ln) || isTableHeader(lines, i)
}

func lastItem(items []*listItem) *listItem {
	if len(items) == 0 {
		return nil
	}
	return items[len(items)-1]
}

// isSameKind reports whether ln is a list marker of the same ordered-ness as the
// list currently being parsed (a switch between bullet and ordered starts a new
// list, handled by the caller breaking out).
func isSameKind(ln string, ordered bool) bool {
	if ordered {
		return reOLMarker.MatchString(ln)
	}
	return reULMarker.MatchString(ln)
}

// stripMarker returns the text after a list marker and the column at which item
// content begins (used to de-indent continuation and nested lines).
func stripMarker(ln string) (content string, contentIndent int) {
	if m := reULMarker.FindStringSubmatch(ln); m != nil {
		return m[4], len(m[1]) + 1 + len(m[3])
	}
	if m := reOLMarker.FindStringSubmatch(ln); m != nil {
		return m[5], len(m[1]) + len(m[2]) + 1 + len(m[4])
	}
	return ln, 0
}

// deindent removes up to n leading spaces from a continuation/nested line.
func deindent(ln string, n int) string {
	i := 0
	for i < n && i < len(ln) && ln[i] == ' ' {
		i++
	}
	return ln[i:]
}

// taskCheckbox detects and strips a leading [ ]/[x] from the first content line,
// returning the checkbox HTML (or "" when the item is not a task item).
func taskCheckbox(it *listItem) string {
	if len(it.lines) == 0 {
		return ""
	}
	ln := it.lines[0]
	if len(ln) >= 3 && ln[0] == '[' && ln[2] == ']' && (len(ln) == 3 || ln[3] == ' ') {
		switch ln[1] {
		case ' ':
			it.lines[0] = strings.TrimPrefix(ln[3:], " ")
			return `<input type="checkbox" disabled> `
		case 'x', 'X':
			it.lines[0] = strings.TrimPrefix(ln[3:], " ")
			return `<input type="checkbox" checked disabled> `
		}
	}
	return ""
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
