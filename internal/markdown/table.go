package markdown

import "strings"

// isTableHeader reports whether the line at i is a GFM table header — a row
// containing a pipe immediately followed by a delimiter row (|---|:--:|).
func isTableHeader(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	if !strings.Contains(lines[i], "|") || strings.TrimSpace(lines[i]) == "" {
		return false
	}
	if reTableSep.MatchString(lines[i]) {
		return false // the line itself is a delimiter, not a header
	}
	return reTableSep.MatchString(lines[i+1])
}

func (r *renderer) table(lines []string, i int) int {
	headers := splitRow(lines[i])
	aligns := tableAligns(lines[i+1])
	i += 2

	r.b.WriteString("<table>\n<thead>\n<tr>\n")
	for c, h := range headers {
		r.b.WriteString("<th")
		r.b.WriteString(alignAttr(aligns, c))
		r.b.WriteString(">")
		r.b.WriteString(r.inline(h))
		r.b.WriteString("</th>\n")
	}
	r.b.WriteString("</tr>\n</thead>\n<tbody>\n")

	for i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != "" {
		cells := splitRow(lines[i])
		r.b.WriteString("<tr>\n")
		for c := range headers {
			cell := ""
			if c < len(cells) {
				cell = cells[c]
			}
			r.b.WriteString("<td")
			r.b.WriteString(alignAttr(aligns, c))
			r.b.WriteString(">")
			r.b.WriteString(r.inline(cell))
			r.b.WriteString("</td>\n")
		}
		r.b.WriteString("</tr>\n")
		i++
	}
	r.b.WriteString("</tbody>\n</table>\n")
	return i
}

// splitRow splits a table row into trimmed cells on unescaped pipes, dropping
// the optional leading/trailing pipe. A pipe inside an inline code span is not a
// delimiter, and backslash parity is honored (\| is escaped, \\| is an escaped
// backslash followed by a delimiter).
func splitRow(ln string) []string {
	ln = strings.TrimSpace(ln)
	ln = strings.TrimPrefix(ln, "|")
	ln = strings.TrimSuffix(ln, "|")
	var cells []string
	var cur strings.Builder
	code := 0 // backtick run length of the open code span, 0 = outside code
	for i := 0; i < len(ln); i++ {
		c := ln[i]
		if c == '`' {
			n := runLen(ln, i, '`')
			switch {
			case code == 0:
				code = n
			case n == code:
				code = 0
			}
			cur.WriteString(ln[i : i+n])
			i += n - 1
			continue
		}
		if c == '\\' && code == 0 {
			j := i
			for j < len(ln) && ln[j] == '\\' {
				j++
			}
			run := j - i
			cur.WriteString(strings.Repeat(`\`, run/2))
			i = j - 1
			if run%2 == 1 {
				if j < len(ln) && ln[j] == '|' {
					cur.WriteByte('|') // \| → literal pipe
					i = j
				} else {
					cur.WriteByte('\\')
				}
			}
			continue
		}
		if c == '|' && code == 0 {
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	return cells
}

func tableAligns(sep string) []string {
	cells := splitRow(sep)
	aligns := make([]string, len(cells))
	for i, c := range cells {
		c = strings.TrimSpace(c)
		left := strings.HasPrefix(c, ":")
		right := strings.HasSuffix(c, ":")
		switch {
		case left && right:
			aligns[i] = "center"
		case right:
			aligns[i] = "right"
		case left:
			aligns[i] = "left"
		}
	}
	return aligns
}

func alignAttr(aligns []string, c int) string {
	if c < len(aligns) && aligns[c] != "" {
		return ` style="text-align:` + aligns[c] + `"`
	}
	return ""
}
