package onepage

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

type renderer struct {
	doc   *doc
	path  string
	b     strings.Builder
	errs  []error
	mores []string
}

func (r *renderer) fail(line int, err error) {
	r.errs = append(r.errs, &SourceError{Path: r.path, Line: line, Err: err})
}

func (r *renderer) w(format string, args ...any) { fmt.Fprintf(&r.b, format, args...) }

func (r *renderer) body() string {
	for _, a := range r.doc.lines {
		r.actor(text{a.actor, a.line})
	}
	for _, a := range r.doc.marks {
		r.actor(text{a.actor, a.line})
	}
	for _, blk := range r.doc.blocks {
		r.w("\n")
		switch b := blk.(type) {
		case *viewHead:
			r.w(`<h2 class="sh"><span class="n">%s</span><span class="t">%s<small>%s</small></span><span class="rule"></span><span class="note">%s</span></h2>`+"\n",
				esc(b.n.s), r.inline(b.title), r.inline(b.sub), r.inline(b.note))
		case *drawing:
			for _, l := range b.lines {
				r.w("%s\n", l.s)
			}
		case *titleblock:
			r.titleblock(b)
		case *details:
			r.details(b)
		case *assembly:
			r.assembly(b)
		case *spec:
			r.spec(b)
		case *swimlane:
			r.swimlane(b)
		}
	}
	if len(r.mores) > 0 {
		r.w("\n<div class=\"mores\" hidden>\n%s\n</div>\n", strings.Join(r.mores, "\n"))
	}
	return r.b.String()
}

func (r *renderer) more(key string, title text, more []text) string {
	if len(more) == 0 {
		return ""
	}
	var ps strings.Builder
	for _, m := range more {
		fmt.Fprintf(&ps, "<p>%s</p>", r.inline(m))
	}
	r.mores = append(r.mores, fmt.Sprintf(`<section id="more-%s"><h3>%s</h3>%s</section>`, esc(key), r.inline(title), ps.String()))
	return fmt.Sprintf(` data-more="%s" tabindex="0"`, esc(key))
}

const moreMark = `<span class="more-mk"></span>`

func (r *renderer) actor(t text) string {
	if t.s == "" {
		return ""
	}
	if !slices.ContainsFunc(r.doc.actors, func(a actorDecl) bool { return a.name == t.s }) {
		r.fail(t.line, fmt.Errorf("%w %q", ErrUnknownActor, t.s))
	}
	return t.s
}

func (r *renderer) titleblock(b *titleblock) {
	r.w("<header class=\"tblock\">\n")
	for _, c := range b.cells {
		r.w("  <div class=\"cell %s\">\n    <span class=\"lab\">%s</span>\n", c.kind, r.inline(c.label))
		for _, el := range c.els {
			switch el.kind {
			case "wordmark":
				r.w("    <div class=\"mark\">%s</div>\n", r.inline(el.text))
			case "motto":
				r.w("    <div class=\"motto\">%s</div>\n", r.inline(el.text))
			case "para":
				r.w("    <p>%s</p>\n", r.inline(el.text))
			case "lead", "big":
				r.w("    <p class=\"%s\">%s</p>\n", el.kind, r.inline(el.text))
			case "plist":
				r.w("    <div class=\"plist\">\n")
				for _, row := range el.rows {
					r.w("      <b>%s</b><span>%s</span><span class=\"see\">%s</span>\n", r.inline(row[0]), r.inline(row[1]), r.inline(row[2]))
				}
				r.w("    </div>\n")
			case "ol":
				r.w("    <ol>\n")
				for _, row := range el.rows {
					r.w("      <li>%s<span>%s</span></li>\n", r.inline(row[0]), r.inline(row[1]))
				}
				r.w("    </ol>\n")
			}
		}
		r.w("  </div>\n")
	}
	r.legend()
	r.w("</header>\n")
}

func (r *renderer) legend() {
	d := r.doc
	r.w("  <div class=\"legend\">\n    <span class=\"k\">WHO ACTS</span>\n    <span class=\"who\">\n")
	for _, a := range d.actors {
		r.w("      <span class=\"%s\"><i></i><b>%s</b><em>%s</em></span>\n", a.name, esc(strings.ToUpper(a.name)), esc(a.label))
	}
	r.w("    </span>\n")
	if len(d.lines) > 0 {
		r.w("    <span class=\"k\">LINES</span>\n")
	}
	for _, l := range d.lines {
		class := strings.TrimSpace(map[string]string{"arrow": "w", "dotted": "d", "planned": "ph"}[l.kind] + " " + l.actor)
		label := esc(l.label)
		switch l.kind {
		case "arrow":
			marker := "a"
			if l.actor != "" {
				marker = "a-" + l.actor
			}
			r.w(`    <span class="sym"><svg width="30" height="10"><line x1="1" y1="5" x2="28" y2="5" class="%s" marker-end="url(#%s)"/></svg>%s</span>`+"\n", class, marker, label)
		case "dotted":
			r.w(`    <span class="sym"><svg width="30" height="10"><line x1="1" y1="5" x2="29" y2="5" class="%s"/></svg>%s</span>`+"\n", class, label)
		case "planned":
			r.w(`    <span class="sym"><svg width="36" height="10"><line x1="1" y1="5" x2="35" y2="5" class="%s"/></svg>%s</span>`+"\n", class, label)
		}
	}
	if len(d.marks) > 0 {
		r.w("    <span class=\"k\">MARKS</span>\n")
	}
	for _, m := range d.marks {
		sample := map[string]string{
			"balloon": `<span class="bal">1</span>`,
			"flag":    `<span class="flag" data-n="1"></span>`,
			"diamond": `<span class="dia" data-n="1"></span>`,
			"more":    moreMark,
		}[m.kind]
		r.w("    <span class=\"sym\">%s%s</span>\n", sample, esc(m.label))
	}
	r.w("  </div>\n")
}

func (r *renderer) details(b *details) {
	r.w("<div class=\"details\">\n")
	for _, p := range b.panels {
		if p.notes {
			r.w("  <div class=\"view notes\">\n    <span class=\"lab\">%s</span>\n    <ol>\n", r.inline(p.title))
			for _, it := range p.items {
				where := ""
				if it[2].s != "" {
					where = fmt.Sprintf(` <span class="where">%s</span>`, r.inline(it[2]))
				}
				r.w("      <li data-note=\"%[1]s\"><span class=\"flag\" data-n=\"%[1]s\"></span><span>%s%s</span></li>\n", esc(it[0].s), r.inline(it[1]), where)
			}
			r.w("    </ol>\n")
			if p.foot.s != "" {
				r.w("    <p class=\"flags-note\">%s</p>\n", r.inline(p.foot))
			}
			r.w("  </div>\n")
			continue
		}
		note := ""
		if p.note.s != "" {
			note = fmt.Sprintf(`<span class="rule-note">%s</span>`, r.inline(p.note))
		}
		r.w("  <div class=\"view\">\n    <span class=\"lab\"><span class=\"tag\">%s</span>%s%s</span>\n", esc(p.tag.s), r.inline(p.title), note)
		if len(p.rungs) > 0 {
			r.ladder(p.rungs)
		}
		r.w("  </div>\n")
	}
	r.w("</div>\n")
}

func (r *renderer) ladder(rungs []*rung) {
	n := len(rungs)
	h := 48*n + 1
	var rails strings.Builder
	fmt.Fprintf(&rails, "M6,0 V%d M28,0 V%d", h, h)
	for i := range n {
		fmt.Fprintf(&rails, " M6,%d H28", 23+i*99/2)
	}
	r.w("    <div class=\"ladder\">\n")
	r.w("      <svg width=\"34\" height=\"%d\" viewBox=\"0 0 34 %d\" style=\"grid-row: 1 / %d\">\n", h, h, n+1)
	r.w("        <path d=\"%s\" class=\"w\"/>\n", rails.String())
	r.w("        <path d=\"M17,%d V8\" class=\"w dash\" marker-end=\"url(#a)\"/>\n", h-8)
	r.w("      </svg>\n")
	for _, g := range slices.Backward(rungs) {
		actor := r.actor(g.actor)
		class, title := actor, r.inline(g.title)
		b := "<b>"
		if g.planned {
			class += " planned"
			b = fmt.Sprintf(`<b class="%s">`, actor)
			title += ` <span class="planned">planned</span>`
		}
		r.w("      <div class=\"rung %s\">%s%s<small>%s</small></b><span>%s</span></div>\n", class, b, title, r.inline(g.who), r.inline(g.s))
	}
	r.w("    </div>\n")
}

func (r *renderer) assembly(b *assembly) {
	r.w("<div class=\"system\">\n")
	r.drawing(b)
	r.w("\n")
	r.w("<div class=\"bom\">\n  <div class=\"ttl\">%s<span>%s</span></div>\n", r.inline(b.title), r.inline(b.sub))
	r.w("  <table>\n    <thead><tr><th>ITEM</th><th>PART · WHAT IT IS</th><th>WHY IT EXISTS</th></tr></thead>\n    <tbody>\n")
	for _, p := range b.parts {
		actor := r.actor(p.actor)
		planned := ""
		if p.planned {
			planned = fmt.Sprintf(` <span class="planned %s">planned</span>`, actor)
		}
		r.w("      <tr data-item=\"%s\"><td><span class=\"bal %s\">%s</span></td><td><b>%s</b>%s <span>%s</span></td><td>%s</td></tr>\n",
			esc(p.n.s), actor, esc(p.n.s), r.inline(p.name), planned, r.inline(p.what), r.inline(p.why))
	}
	r.w("    </tbody>\n  </table>\n</div>\n</div>\n")
}

func (r *renderer) spec(b *spec) {
	r.w("<div class=\"spec\">\n")
	for i, col := range b.cols {
		if i > 0 {
			r.w("\n")
		}
		r.w("  <div class=\"col\">\n")
		for _, h := range col {
			opens, mark := r.more(h.no.s, h.to, h.more), ""
			if opens != "" {
				mark = moreMark
			}
			r.w("    <div class=\"item\"%s>\n", opens)
			r.w("      <div class=\"hdr\"><span class=\"no\">%s</span><span class=\"to\">%s</span>%s<span class=\"ty\">%s</span></div>\n",
				esc(h.no.s), r.inline(h.to), mark, r.typed(h.typed))
			for j := 0; j < len(h.content); j++ {
				switch c := h.content[j].(type) {
				case text:
					r.w("      <p class=\"cap\">%s</p>\n", r.inline(c))
				case *term:
					if cap, ok := next(h.content, j); c.beside && ok {
						r.w("      <div class=\"inline\">%s<p class=\"cap\">%s</p></div>\n", r.term(c), r.inline(cap))
						j++
					} else {
						r.w("%s\n", r.term(c))
					}
				}
			}
			r.w("    </div>\n")
		}
		r.w("  </div>\n")
	}
	r.w("</div>\n")
}

func next(content []any, i int) (text, bool) {
	if i+1 >= len(content) {
		return text{}, false
	}
	t, ok := content[i+1].(text)
	return t, ok
}

func (r *renderer) typed(t text) string {
	var b strings.Builder
	s := t.s
	for s != "" {
		open := strings.IndexByte(s, '`')
		end := -1
		if open >= 0 {
			end = strings.IndexByte(s[open+1:], '`')
		}
		if open < 0 || end < 0 {
			if w := strings.TrimSpace(s); w != "" {
				fmt.Fprintf(&b, "<em>%s</em>", r.inline(text{w, t.line}))
			}
			break
		}
		if w := strings.TrimSpace(s[:open]); w != "" {
			fmt.Fprintf(&b, "<em>%s</em>", r.inline(text{w, t.line}))
		}
		code := s[open+1 : open+1+end]
		s = s[open+end+2:]
		class := ""
		if rest := strings.TrimLeft(s, " "); strings.HasPrefix(rest, "@") {
			name, after, _ := strings.Cut(rest[1:], " ")
			class = fmt.Sprintf(` class="%s"`, r.actor(text{name, t.line}))
			s = after
		}
		fmt.Fprintf(&b, "<code%s>%s</code>", class, esc(code))
	}
	return b.String()
}

func (r *renderer) inline(t text) string {
	var b strings.Builder
	s := t.s
	for s != "" {
		switch {
		case s[0] == '\\' && len(s) > 1:
			_, size := utf8.DecodeRuneInString(s[1:])
			b.WriteString(esc(s[1 : 1+size]))
			s = s[1+size:]
			continue
		case strings.HasPrefix(s, "**"):
			if end := strings.Index(s[2:], "**"); end >= 0 {
				fmt.Fprintf(&b, "<b>%s</b>", r.inline(text{s[2 : 2+end], t.line}))
				s = s[4+end:]
				continue
			}
		case s[0] == '`':
			if end := strings.IndexByte(s[1:], '`'); end >= 0 {
				fmt.Fprintf(&b, "<code>%s</code>", esc(s[1:1+end]))
				s = s[2+end:]
				continue
			}
		case s[0] == '@':
			name := leading(s[1:], func(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' })
			rest := s[1+len(name):]
			if name != "" && strings.HasPrefix(rest, "{") {
				if end := closing(rest); end > 0 {
					fmt.Fprintf(&b, `<span class="%s">%s</span>`, r.actor(text{name, t.line}), r.inline(text{rest[1:end], t.line}))
					s = rest[end+1:]
					continue
				}
			}
		case s[0] == '{':
			n := leading(s[1:], isDigit)
			if n != "" && strings.HasPrefix(s[1+len(n):], "}") {
				actor, ok := r.doc.parts[n]
				if !ok {
					r.fail(t.line, fmt.Errorf("%w {%s}", ErrUnknownPart, n))
				}
				fmt.Fprintf(&b, `<span class="%s" data-item="%s">%s</span>`, strings.TrimSpace("bal "+actor), n, n)
				s = s[2+len(n):]
				continue
			}
		case strings.HasPrefix(s, "▲"), strings.HasPrefix(s, "◆"):
			n := leading(s[len("▲"):], isDigit)
			if n != "" {
				mark := fmt.Sprintf(`class="dia" data-n="%s"`, n)
				if strings.HasPrefix(s, "▲") {
					mark = fmt.Sprintf(`class="flag" data-n="%[1]s" data-note="%[1]s"`, n)
					if !r.doc.notes[n] {
						r.fail(t.line, fmt.Errorf("%w ▲%s", ErrUnknownNote, n))
					}
				} else if !r.doc.diamonds[n] {
					r.fail(t.line, fmt.Errorf("%w ◆%s", ErrUnknownDiamond, n))
				}
				fmt.Fprintf(&b, `<span %s></span>`, mark)
				s = s[len("▲")+len(n):]
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(s)
		b.WriteString(esc(s[:size]))
		s = s[size:]
	}
	return b.String()
}

func closing(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func leading(s string, ok func(byte) bool) string {
	i := 0
	for i < len(s) && ok(s[i]) {
		i++
	}
	return s[:i]
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

var termMarks = []string{"p", "c", "k", "i", "h", "ok", "q", "a"}

func (r *renderer) term(t *term) string {
	lines := make([]string, len(t.lines))
	for i, l := range t.lines {
		s, out := l.s, ""
		if cmd, ok := strings.CutPrefix(s, "$ "); ok {
			out, s = `<span class="p">$</span> <span class="c">`+r.termMarks(text{cmd, l.line})+`</span>`, ""
		} else if indent := len(s) - len(strings.TrimLeft(s, " ")); indent <= 3 {
			if key := leading(s[indent:], isLower); key != "" && strings.HasPrefix(s[indent+len(key):], " ") {
				out = s[:indent] + `<span class="k">` + key + `</span>`
				s = s[indent+len(key):]
			}
		}
		lines[i] = out + r.termMarks(text{s, l.line})
	}
	return `<pre class="term">` + strings.Join(lines, "\n") + `</pre>`
}

func (r *renderer) termMarks(t text) string {
	var b strings.Builder
	s := t.s
	for s != "" {
		if strings.HasPrefix(s, `\{`) {
			b.WriteString("{")
			s = s[2:]
			continue
		}
		if s[0] == '{' {
			name := leading(s[1:], isLower)
			if name != "" && strings.HasPrefix(s[1+len(name):], ":") {
				if !slices.Contains(termMarks, name) {
					r.fail(t.line, fmt.Errorf("%w {%s:}, not one of %s", ErrUnknownMark, name, strings.Join(termMarks, ", ")))
				}
				if end := strings.IndexByte(s, '}'); end > 0 {
					fmt.Fprintf(&b, `<span class="%s">%s</span>`, name, esc(s[2+len(name):end]))
					s = s[end+1:]
					continue
				}
			}
		}
		_, size := utf8.DecodeRuneInString(s)
		b.WriteString(esc(s[:size]))
		s = s[size:]
	}
	return b.String()
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func esc(s string) string { return escaper.Replace(s) }
