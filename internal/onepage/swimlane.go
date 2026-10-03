package onepage

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	sheetWidth     = 1500
	headRow        = 30
	gap            = 20
	textGap        = 4
	laneTop        = 8
	laneBottom     = 12
	phasePad       = 10
	rail           = 12
	padLeft        = 9
	padRight       = 10
	stackX, stackY = 8, 6
	clear          = 6
)

type run struct {
	s, class string
	marks    []drawnMark
}

const chipPad, chipGap = 4, 3

func (rn run) advance() float64 {
	switch rn.kind() {
	case "chip":
		return max(18, textWidth(rn.s, "tc")+2*chipPad)
	case "page":
		return 13
	}
	return textWidth(rn.s, rn.class)
}

func (rn run) kind() string {
	k, _, _ := strings.Cut(rn.class, " ")
	if k == "chip" || k == "page" {
		return k
	}
	return ""
}

func (l sline) reach(x float64) ([]float64, float64) {
	for _, m := range l.lead {
		x += float64(m.width() + 4)
	}
	at := make([]float64, len(l.runs))
	for i, rn := range l.runs {
		if i > 0 && l.runs[i-1].kind() != "" {
			x += chipGap
		}
		at[i] = x
		x += rn.advance()
	}
	return at, x
}

type drawnMark struct{ class, n, actor string }

func (m drawnMark) width() int { return map[string]int{"dia": 16, "flag": 18, "bal": 20}[m.class] }

type sline struct {
	line        int
	runs        []run
	lead, trail []drawnMark
	size        int
}

func (l sline) width() float64 {
	_, w := l.reach(0)
	for _, m := range l.trail {
		w += float64(m.width() + 4)
	}
	return w
}

type placed struct {
	*item
	laneN, phaseN int
	lines         []sline
	rows          []sline
	natural       int
	x, y, w, h    int
}

type rect struct{ x0, y0, x1, y1 int }

func (p *placed) rect() rect { return rect{p.x, p.y, p.x + p.w, p.y + p.h} }

func (r *renderer) swimlane(s *swimlane) {
	before := len(r.errs)
	laneOf, phaseOf := map[string]int{}, map[string]int{}
	for i, l := range s.lanes {
		laneOf[r.actor(l.actor)] = i
	}
	for i, p := range s.phases {
		phaseOf[p.n.s] = i
	}
	var items []*placed
	byItem := map[*item]*placed{}
	for _, it := range s.items {
		li, ok := laneOf[it.lane.s]
		if !ok {
			r.fail(it.line, fmt.Errorf("%w %q", ErrUnknownLane, it.lane.s))
			continue
		}
		pi, ok := phaseOf[it.phase.s]
		if !ok {
			r.fail(it.line, fmt.Errorf("%w %q", ErrUnknownPhase, it.phase.s))
			continue
		}
		p := &placed{item: it, laneN: li, phaseN: pi}
		base, pad := "ts", padLeft+padRight
		if it.kind == "text" {
			base, pad = "ti", padLeft
		}
		width := 0.0
		for _, t := range it.lines {
			l := r.sline(t, base, it.lane.s)
			p.lines = append(p.lines, l)
			width = max(width, l.width())
		}
		p.natural = int(math.Ceil(width)) + pad
		p.h = height(p.lines, it.kind == "text")
		if it.kind == "stack" {
			p.natural += stackX
			p.h += stackY
		}
		items = append(items, p)
		byItem[it] = p
	}
	box := func(t text) *placed {
		it, ok := s.boxes[t.s]
		if !ok {
			r.fail(t.line, fmt.Errorf("%w %q", ErrUnknownBox, t.s))
			return nil
		}
		return byItem[it]
	}
	type route struct {
		a        *arrow
		from, to *placed
	}
	var routes []route
	for _, a := range s.arrows {
		from, to := box(a.from), box(a.to)
		if a.from.s == a.to.s {
			r.fail(a.line, fmt.Errorf("%w: an arrow joins two boxes, not %q to itself", ErrFields, a.from.s))
		}
		if from != nil && to != nil && from != to {
			routes = append(routes, route{a, from, to})
		}
	}
	type framed struct {
		f     *frame
		boxes []*placed
	}
	var frames []framed
	for _, f := range s.frames {
		if !r.doc.details[f.letter.s] {
			r.fail(f.letter.line, fmt.Errorf("%w %q", ErrUnknownDetail, f.letter.s))
		}
		fr := framed{f: f}
		for _, t := range f.boxes {
			if p := box(t); p != nil {
				fr.boxes = append(fr.boxes, p)
			}
		}
		frames = append(frames, fr)
	}
	if len(r.errs) > before {
		return
	}

	cols := make([][]int, len(s.phases))
	filled := make([][]bool, len(s.phases))
	for i := range cols {
		n := 1
		for _, p := range items {
			if p.phaseN == i {
				n = max(n, p.col1)
			}
		}
		cols[i], filled[i] = make([]int, n), make([]bool, n)
	}
	for _, p := range items {
		if p.col0 == p.col1 {
			c := p.col0 - 1
			cols[p.phaseN][c] = max(cols[p.phaseN][c], p.natural)
			filled[p.phaseN][c] = true
		}
	}
	for i := range cols {
		for c := range cols[i] {
			if !filled[i][c] {
				cols[i][c] = rail
			}
		}
	}
	grow := func(i, first, last, need int) { widen(cols[i], filled[i], first, last, need) }
	spans := slices.Clone(items)
	slices.SortStableFunc(spans, func(a, b *placed) int { return (a.col1 - a.col0) - (b.col1 - b.col0) })
	for _, p := range spans {
		if p.col0 == p.col1 {
			continue
		}
		have := gap * (p.col1 - p.col0)
		for c := p.col0 - 1; c < p.col1; c++ {
			have += cols[p.phaseN][c]
		}
		if p.natural > have {
			grow(p.phaseN, p.col0-1, p.col1-1, p.natural-have)
		}
	}
	inner := func(i int) int {
		w := gap * (len(cols[i]) - 1)
		for _, c := range cols[i] {
			w += c
		}
		return w
	}
	for i, ph := range s.phases {
		if need := 34 + int(math.Ceil(textWidth(ph.title.s, "tc"))) + 8 - 2*phasePad - inner(i); need > 0 {
			grow(i, 0, len(cols[i])-1, need)
		}
	}

	labels := make([][]text, len(s.lanes))
	labelW := 0.0
	for i, l := range s.lanes {
		labels[i] = l.lines
		if len(l.lines) == 0 {
			for _, a := range r.doc.actors {
				if a.name == l.actor.s {
					labels[i] = []text{{a.label, l.actor.line}}
				}
			}
		}
		labelW = max(labelW, textWidth(strings.ToUpper(l.actor.s), "tcl"))
		for _, t := range labels[i] {
			labelW = max(labelW, textWidth(t.s, "ts"))
		}
	}
	left := 16 + int(math.Ceil(labelW)) + 14

	total := left
	widths := []string{fmt.Sprintf("the lanes' names %d", left)}
	for i, ph := range s.phases {
		total += inner(i) + 2*phasePad
		widths = append(widths, fmt.Sprintf("phase %s %d", ph.n.s, inner(i)+2*phasePad))
	}
	if total > sheetWidth {
		r.fail(s.line, fmt.Errorf("%w: the swimlane needs %dpx, the sheet has %d: %s", ErrTooWide, total, sheetWidth, strings.Join(widths, ", ")))
		return
	}
	var wide [][2]int
	for i := range cols {
		for c := range cols[i] {
			if filled[i][c] {
				wide = append(wide, [2]int{i, c})
			}
		}
	}
	if slack := sheetWidth - total; len(wide) > 0 {
		for j, ic := range wide {
			cols[ic[0]][ic[1]] += slack / len(wide)
			if j < slack%len(wide) {
				cols[ic[0]][ic[1]]++
			}
		}
	} else if len(cols) > 0 {
		last := len(cols) - 1
		cols[last][len(cols[last])-1] += slack
	}
	starts := make([]int, len(s.phases))
	colX := make([][]int, len(s.phases))
	x := left
	for i := range cols {
		starts[i] = x
		x += phasePad
		for _, w := range cols[i] {
			colX[i] = append(colX[i], x)
			x += w + gap
		}
		x += phasePad - gap
	}

	tops := make([]int, len(s.lanes)+1)
	tops[0] = headRow
	for li := range s.lanes {
		top, bottom := tops[li], tops[li]
		var done []*placed
		for _, p := range items {
			if p.laneN != li {
				continue
			}
			p.x = colX[p.phaseN][p.col0-1]
			p.w = colX[p.phaseN][p.col1-1] + cols[p.phaseN][p.col1-1] - p.x
			p.y = top + laneTop
			under := gap
			if p.kind == "text" {
				under = textGap
			}
			for _, q := range done {
				if q.x < p.x+p.w && p.x < q.x+q.w {
					p.y = max(p.y, q.y+q.h+under)
				}
			}
			done = append(done, p)
			bottom = max(bottom, p.y+p.h+laneBottom)
		}
		label := 28 + 16
		if n := len(labels[li]); n > 0 {
			label += 18 + 14*(n-1)
		}
		tops[li+1] = max(bottom, top+label)
	}
	height := tops[len(s.lanes)]

	r.w("<svg class=\"swimlane\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", sheetWidth, height, sheetWidth, height)
	var hair strings.Builder
	for li, l := range s.lanes {
		r.w("  <rect x=\"0\" y=\"%d\" width=\"%d\" height=\"%d\" class=\"lane %s\"/>\n", tops[li], sheetWidth, tops[li+1]-tops[li], l.actor.s)
		fmt.Fprintf(&hair, "M0,%d H%d ", tops[li], sheetWidth)
	}
	fmt.Fprintf(&hair, "M0,%d H%d M%d,0 V%d", height, sheetWidth, left, height)
	r.w("  <path d=\"%s\" class=\"hair\"/>\n", hair.String())
	for li, l := range s.lanes {
		top := tops[li]
		r.w("  <rect x=\"0\" y=\"%d\" width=\"5\" height=\"%d\" class=\"f-%s\"/>\n", top, tops[li+1]-top, l.actor.s)
		r.w("  <text x=\"16\" y=\"%d\" class=\"tcl f-%s\">%s</text>", top+28, l.actor.s, esc(strings.ToUpper(l.actor.s)))
		for j, t := range labels[li] {
			r.w("<text x=\"16\" y=\"%d\" class=\"ts\">%s</text>", top+46+14*j, esc(t.s))
		}
		r.w("\n")
	}
	for i, ph := range s.phases {
		if i > 0 {
			r.w("  <path d=\"M%d,4 V%d\" class=\"sep\"/>\n", starts[i], height)
		}
		r.w("  <circle cx=\"%d\" cy=\"15\" r=\"10\" class=\"bx\"/><text x=\"%d\" y=\"19\" class=\"tc mid\">%s</text><text x=\"%d\" y=\"19\" class=\"tc\">%s</text>\n",
			starts[i]+18, starts[i]+18, esc(ph.n.s), starts[i]+34, esc(ph.title.s))
	}

	for _, p := range items {
		r.item(p)
	}

	pair := func(a, b *placed) [2]*placed {
		if a.line > b.line {
			a, b = b, a
		}
		return [2]*placed{a, b}
	}
	count, seen := map[[2]*placed]int{}, map[[2]*placed]int{}
	for _, rt := range routes {
		count[pair(rt.from, rt.to)]++
	}
	for _, rt := range routes {
		var others []rect
		for _, p := range items {
			if p != rt.from && p != rt.to {
				others = append(others, p.rect())
			}
		}
		k := pair(rt.from, rt.to)
		class, marker := arrowStyle(rt.from.lane.s, rt.a.style)
		need := 0
		if marker != "" {
			need = room(class)
		}
		d, ok := way(rt.from.rect(), rt.to.rect(), others, seen[k], count[k], need)
		if !ok {
			r.fail(rt.a.line, fmt.Errorf("%w from %q to %q: a box stands in every way; an empty column is a rail to run it down", ErrNoWay, rt.a.from.s, rt.a.to.s))
		}
		seen[k]++
		if marker != "" {
			r.arrive(rt.a.line, d, class, false, rt.a.from.s, rt.a.to.s)
		}
		r.w("  <path d=\"%s\" class=\"%s\"%s/>\n", d, class, marker)
	}

	for _, fr := range frames {
		if len(fr.boxes) == 0 {
			continue
		}
		u := fr.boxes[0].rect()
		for _, p := range fr.boxes[1:] {
			b := p.rect()
			u = rect{min(u.x0, b.x0), min(u.y0, b.y0), max(u.x1, b.x1), max(u.y1, b.y1)}
		}
		r.w("  <rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" rx=\"8\" class=\"detail\"/>\n", u.x0-5, u.y0-5, u.x1-u.x0+10, u.y1-u.y0+10)
		r.w("  <g class=\"dtag\" transform=\"translate(%d,%d)\"><circle r=\"11\"/><text y=\"4.5\">%s</text></g>\n", u.x0-5, u.y1+5, esc(fr.f.letter.s))
	}
	r.w("</svg>\n")
}

func widen(sizes []int, filled []bool, first, last, need int) {
	var at []int
	for c := first; c <= last; c++ {
		if filled[c] {
			at = append(at, c)
		}
	}
	if len(at) == 0 {
		for c := first; c <= last; c++ {
			at = append(at, c)
		}
	}
	for j, c := range at {
		sizes[c] += need / len(at)
		if j < need%len(at) {
			sizes[c]++
		}
	}
}

func height(lines []sline, bare bool) int {
	h := 16 + 8
	if bare {
		h = 11 + 4
	}
	for i := 1; i < len(lines); i++ {
		h += max(lines[i-1].size, lines[i].size)
	}
	return h
}

func (r *renderer) item(p *placed) {
	if len(p.more) > 0 {
		r.w("  <g class=\"opens\"%s>\n", r.more(p.name, p.item.lines[0], p.more))
		defer r.w("  </g>\n")
	}
	x, y, w, h := p.x, p.y, p.w, p.h
	class := "bx " + p.lane.s
	switch p.kind {
	case "chip":
		class += " chip"
	case "note":
		class = "bx note"
	}
	if p.planned {
		class += " ph"
	}
	switch p.kind {
	case "text":
	case "stack":
		r.w("  <g class=\"stack\">")
		for i := range 3 {
			r.w("<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" class=\"%s\"/>", x+4*i, y+3*i, w-stackX, h-stackY, class)
		}
		r.w("</g>\n")
		x, y, w = x+stackX, y+stackY, w-stackX
	default:
		r.w("  <rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" class=\"%s\"/>\n", x, y, w, h, class)
	}
	base := y + 16
	if p.kind == "text" {
		base = y + 11
	}
	r.lines(p.lines, base, func(sline) (int, int) { return x, w })
	if len(p.more) > 0 {
		r.w("  <g class=\"more-mk\" transform=\"translate(%d,%d)\"><rect x=\"-6\" y=\"-6\" width=\"12\" height=\"12\"/><path d=\"M-3.5,0 H3.5 M0,-3.5 V3.5\"/></g>\n", x+w, y)
	}
}

func (r *renderer) lines(ls []sline, base int, place func(sline) (x, w int)) {
	for i, l := range ls {
		if i > 0 {
			base += max(ls[i-1].size, l.size)
		}
		x, w := place(l)
		r.line(l, x, w, base)
	}
}

func arrowStyle(actor, style string) (class, marker string) {
	head := "a"
	if actor != "" {
		head += "-" + actor
	}
	marker = fmt.Sprintf(` marker-end="url(#%s)"`, head)
	switch style {
	case "dotted":
		return strings.TrimSpace("d " + actor), ""
	case "planned":
		return strings.TrimSpace("ph " + actor), marker
	}
	return strings.TrimSpace("w " + actor), marker
}

func (r *renderer) line(l sline, x, w, base int) {
	r.w("  ")
	tx := x + padLeft
	for _, m := range l.lead {
		r.w("%s", r.markSVG(m, tx+m.width()/2, base-4))
		tx += m.width() + 4
	}
	starts, _ := l.reach(float64(x + padLeft))
	words := func(rs []run, x0 float64) {
		switch len(rs) {
		case 0:
		case 1:
			r.w("<text x=\"%d\" y=\"%d\" class=\"%s\">%s</text>", int(math.Round(x0)), base, rs[0].class, esc(rs[0].s))
		default:
			r.w("<text x=\"%d\" y=\"%d\">", int(math.Round(x0)), base)
			for _, rn := range rs {
				r.w("<tspan class=\"%s\">%s</tspan>", rn.class, esc(rn.s))
			}
			r.w("</text>")
		}
	}
	from := 0
	for i, rn := range l.runs {
		if rn.kind() == "" {
			continue
		}
		words(l.runs[from:i], starts[from])
		from = i + 1
		cx, cw := int(math.Round(starts[i])), int(math.Round(rn.advance()))
		colour := strings.TrimPrefix(rn.class, rn.kind())
		if rn.kind() == "page" {
			r.w("<path d=\"M%d,%d h9 l4,4 v13 h-13 Z M%d,%d v4 h4\" class=\"leaf%s\"/>", cx, base-13, cx+9, base-13, colour)
			continue
		}
		r.w("<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"15\" class=\"pip%s\"/><text x=\"%d\" y=\"%d\" class=\"pipt%s\">%s</text>",
			cx, base-12, cw, colour, cx+cw/2, base, colour, esc(rn.s))
	}
	if from < len(l.runs) {
		words(l.runs[from:], starts[from])
	}
	mx := x + w - padRight
	for _, m := range slices.Backward(l.trail) {
		r.w("%s", r.markSVG(m, mx-m.width()/2, base-4))
		mx -= m.width() + 4
	}
	r.w("\n")
}

func (r *renderer) markSVG(m drawnMark, x, y int) string {
	switch m.class {
	case "dia":
		return fmt.Sprintf(`<g class="dia" transform="translate(%d,%d)"><path d="M0,-8 L8,0 L0,8 L-8,0 Z"/><text y="3.5">%s</text></g>`, x, y, m.n)
	case "flag":
		return fmt.Sprintf(`<g class="flag" data-note="%s" transform="translate(%d,%d)"><path d="M0,-8 L9,8 L-9,8 Z"/><text y="6">%[1]s</text></g>`, m.n, x, y)
	}
	return fmt.Sprintf(`<g class="%s" data-item="%s" transform="translate(%d,%d)"><circle r="10"/><text y="4">%s</text></g>`, strings.TrimSpace("bal "+m.actor), m.n, x, y, m.n)
}

func (r *renderer) sline(t text, base, lane string) sline {
	l := sline{line: t.line, size: 15}
	switch {
	case base == "ti":
		l.size = 14
	case strings.ContainsFunc(t.s, unicode.IsUpper) && !strings.ContainsFunc(t.s, unicode.IsLower):
		base, l.size = "tc", 17
		if lane != "" {
			base += " f-" + lane
		}
	}
	r.runs(&l, t.s, t.line, base, "")
	if n := len(l.runs); n > 0 {
		l.runs[0].s = strings.TrimLeft(l.runs[0].s, " ")
		l.runs[n-1].s = strings.TrimRight(l.runs[n-1].s, " ")
		l.runs = slices.DeleteFunc(l.runs, func(rn run) bool { return rn.s == "" && rn.kind() == "" })
	}
	for i, rn := range l.runs {
		if rn.kind() != "" && lane != "" {
			l.runs[i].class = rn.kind() + " " + lane
		}
		l.trail = append(l.trail, rn.marks...)
	}
	for _, m := range slices.Concat(l.lead, l.trail) {
		l.size = max(l.size, map[bool]int{true: 22, false: 18}[m.class == "bal"])
	}
	return l
}

func (r *renderer) runs(l *sline, s string, line int, class, colour string) {
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			l.runs = append(l.runs, run{s: b.String(), class: strings.TrimSpace(class + " " + colour)})
			b.Reset()
		}
	}
	words := func() bool {
		return strings.TrimSpace(b.String()) != "" || slices.ContainsFunc(l.runs, func(rn run) bool { return strings.TrimSpace(rn.s) != "" })
	}
	for s != "" {
		switch {
		case s[0] == '\\' && len(s) > 1:
			_, size := utf8.DecodeRuneInString(s[1:])
			b.WriteString(s[1 : 1+size])
			s = s[1+size:]
			continue
		case strings.HasPrefix(s, "**"):
			if end := strings.Index(s[2:], "**"); end >= 0 {
				flush()
				r.runs(l, s[2:2+end], line, styled(class, "tb"), colour)
				s = s[4+end:]
				continue
			}
		case s[0] == '`':
			if end := strings.IndexByte(s[1:], '`'); end >= 0 {
				flush()
				l.runs = append(l.runs, run{s: s[1 : 1+end], class: strings.TrimSpace(styled(class, "tm") + " " + colour)})
				s = s[2+end:]
				continue
			}
		case s[0] == '[':
			end := strings.IndexByte(s, ']')
			if end < 0 {
				r.fail(line, fmt.Errorf("%w: a chip opened with [ and never closed", ErrUnknownMark))
				s = s[1:]
				continue
			}
			flush()
			var in sline
			r.runs(&in, s[1:end], line, "tc", "")
			var words strings.Builder
			for _, rn := range in.runs {
				words.WriteString(rn.s)
			}
			l.runs = append(l.runs, run{strings.TrimSpace(words.String()), "chip", slices.Concat(in.lead, in.trail)})
			s = s[end+1:]
			continue
		case strings.HasPrefix(s, "📄"):
			flush()
			l.runs = append(l.runs, run{s: "", class: "page"})
			s = strings.TrimPrefix(s[len("📄"):], " ")
			continue
		case s[0] == '@':
			name := leading(s[1:], func(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' })
			rest := s[1+len(name):]
			if name != "" && strings.HasPrefix(rest, "{") {
				if end := closing(rest); end > 0 {
					flush()
					r.runs(l, rest[1:end], line, class, "f-"+r.actor(text{name, line}))
					s = rest[end+1:]
					continue
				}
			}
		case s[0] == '{', strings.HasPrefix(s, "▲"), strings.HasPrefix(s, "◆"):
			var m drawnMark
			var size int
			if s[0] == '{' {
				n := leading(s[1:], isDigit)
				if n != "" && strings.HasPrefix(s[1+len(n):], "}") {
					actor, ok := r.doc.parts[n]
					if !ok {
						r.fail(line, fmt.Errorf("%w {%s}", ErrUnknownPart, n))
					}
					m, size = drawnMark{"bal", n, actor}, 2+len(n)
				}
			} else if n := leading(s[len("▲"):], isDigit); n != "" {
				m, size = drawnMark{"dia", n, ""}, len("▲")+len(n)
				if strings.HasPrefix(s, "▲") {
					m.class = "flag"
					if !r.doc.notes[n] {
						r.fail(line, fmt.Errorf("%w ▲%s", ErrUnknownNote, n))
					}
				} else if !r.doc.diamonds[n] {
					r.fail(line, fmt.Errorf("%w ◆%s", ErrUnknownDiamond, n))
				}
			}
			if size > 0 {
				if words() {
					l.trail = append(l.trail, m)
				} else {
					l.lead = append(l.lead, m)
				}
				s = s[size:]
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(s)
		b.WriteString(s[:size])
		s = s[size:]
	}
	flush()
}

func styled(class, style string) string {
	if class == "ts" {
		return style
	}
	return class + " " + style
}

func way(a, b rect, others []rect, k, n, room int) (string, bool) {
	spread := func(lo, hi int) int { return lo + (hi-lo)*(k+1)/(n+1) }
	if lo, hi := max(a.x0, b.x0), min(a.x1, b.x1); lo < hi {
		y0, y1, from, to := a.y1, b.y0-2, a.y1, b.y0
		if b.y1 <= a.y0 {
			y0, y1, from, to = a.y0, b.y1+2, b.y1, a.y0
		}
		var covers [][2]int
		for _, o := range others {
			if o.y0 < to && from < o.y1 {
				covers = append(covers, [2]int{o.x0 - clear, o.x1 + clear})
			}
		}
		lo, hi, ok := open(lo, hi, covers)
		return fmt.Sprintf("M%d,%d V%d", spread(lo, hi), y0, y1), ok
	}
	if lo, hi := max(a.y0, b.y0), min(a.y1, b.y1); lo < hi {
		x0, x1, from, to := a.x1, b.x0-2, a.x1, b.x0
		if b.x1 <= a.x0 {
			x0, x1, from, to = a.x0, b.x1+2, b.x1, a.x0
		}
		var covers [][2]int
		for _, o := range others {
			if o.x0 < to && from < o.x1 {
				covers = append(covers, [2]int{o.y0 - clear, o.y1 + clear})
			}
		}
		lo, hi, ok := open(lo, hi, covers)
		return fmt.Sprintf("M%d,%d H%d", x0, spread(lo, hi), x1), ok
	}
	blocked := func(x0, y0, x1, y1 int) bool {
		x0, x1 = min(x0, x1), max(x0, x1)
		y0, y1 = min(y0, y1), max(y0, y1)
		return slices.ContainsFunc(others, func(o rect) bool {
			return o.x0-clear < x1 && x0 < o.x1+clear && o.y0-clear < y1 && y0 < o.y1+clear
		})
	}
	ax, ay, bx, by := spread(a.x0, a.x1), spread(a.y0, a.y1), spread(b.x0, b.x1), spread(b.y0, b.y1)
	sideA, sideB := a.x1, b.x0-2
	if b.x1 <= a.x0 {
		sideA, sideB = a.x0, b.x1+2
	}
	edgeA, edgeB := a.y1, b.y0-2
	if b.y1 <= a.y0 {
		edgeA, edgeB = a.y0, b.y1+2
	}
	across := !blocked(sideA, ay, bx, ay) && !blocked(bx, ay, bx, edgeB)
	down := !blocked(ax, edgeA, ax, by) && !blocked(ax, by, sideB, by)
	if across && (abs(edgeB-ay) >= room || !down || abs(sideB-ax) < room) {
		return fmt.Sprintf("M%d,%d H%d V%d", sideA, ay, bx, edgeB), true
	}
	return fmt.Sprintf("M%d,%d V%d H%d", ax, edgeA, by, sideB), down
}

func open(lo, hi int, covers [][2]int) (int, int, bool) {
	slices.SortFunc(covers, func(a, b [2]int) int { return a[0] - b[0] })
	best, bestLo, bestHi, x := 0, lo, hi, lo
	for _, c := range append(covers, [2]int{hi, hi}) {
		if end := min(c[0], hi); end-x > best {
			best, bestLo, bestHi = end-x, x, end
		}
		x = max(x, c[1])
	}
	return bestLo, bestHi, best > 0
}

var glyphs = [...]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

func textWidth(s, class string) float64 {
	em := 0.0
	n := 0
	for _, c := range s {
		n++
		switch {
		case c >= ' ' && int(c-' ') < len(glyphs):
			em += float64(glyphs[c-' ']) / 1000
		case c == '→' || c == '—' || c == '…':
			em += 1
		default:
			em += 0.6
		}
	}
	fields := strings.Fields(class)
	switch {
	case slices.Contains(fields, "tm"):
		return 11.5 * 0.61 * float64(n)
	case slices.Contains(fields, "tcl"):
		return 17 * (em*0.86 + 0.1*float64(n))
	case slices.Contains(fields, "tc"):
		return 12 * (em*0.86 + 0.08*float64(n))
	case slices.Contains(fields, "tb"):
		return 12 * em * 1.07 * 1.07
	case slices.Contains(fields, "ts"):
		return 11.5 * em * 1.07
	}
	return 12 * em * 1.07
}
