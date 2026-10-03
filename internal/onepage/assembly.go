package onepage

import (
	"cmp"
	"container/heap"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
)

const (
	systemWidth = 960
	cellGap     = 48
	edge        = 28
	groupPad    = 14
	tabInset    = 10
	track       = 8
	lid         = 10
	figure      = 72
	hop         = 4
	bendCost    = 24
	crossCost   = 60
	balloonR    = 10
	cmpH        = 26
	cmpGap      = 5
	cmpInset    = 14
	labelPitch  = 14
	railTurn    = 12
)

const (
	east = iota
	south
	west
	north
)

type point struct{ x, y int }

var step = [4]point{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}

type seg struct{ a, b point }

func (s seg) vertical() bool { return s.a.x == s.b.x }

func segOf(a, b point) seg {
	if b.x < a.x || b.y < a.y {
		a, b = b, a
	}
	return seg{a, b}
}

type conn struct {
	a        *arrow
	from, to *placed
	out, in  int
	p, q     point
	way      []point
	bundle   *conn
	start    point
	from0    int
	via      *valve
	label    *label
}

func (r *renderer) drawing(a *assembly) {
	if len(a.items) == 0 {
		return
	}
	before := len(r.errs)
	var all, parts, groups []*placed
	var callouts []*callout
	byItem := map[*item]*placed{}
	for _, it := range a.items {
		r.actor(it.lane)
		p := &placed{item: it}
		width := 0.0
		var rows []sline
		for i, t := range it.lines {
			l := r.sline(t, "ts", it.lane.s)
			if row := chipsOnly(l); it.kind == "store" && compartments(row) {
				rows = append(rows, row)
				continue
			}
			for _, ms := range []*[]drawnMark{&l.lead, &l.trail} {
				*ms = slices.DeleteFunc(*ms, func(m drawnMark) bool {
					if m.class == "bal" {
						callouts = append(callouts, &callout{m: m, p: p, i: len(p.lines), corner: i == 0})
					}
					return m.class == "bal"
				})
			}
			p.lines = append(p.lines, l)
			width = max(width, l.width())
		}
		p.rows = rows
		w := int(math.Ceil(width))
		switch it.kind {
		case "group":
			p.natural = tabInset + w + 12 + 10 - 2*groupPad
		case "person":
			p.natural, p.h = max(60, w)+padLeft+padRight, figure+height(p.lines, false)
		case "rail":
			p.natural, p.h = max(0, 2*(6+w+padRight+clear-cellGap)), 0
		default:
			p.natural, p.h = w+padLeft+padRight, height(p.lines, false)
		}
		switch it.kind {
		case "stack":
			p.natural, p.h = p.natural+stackX, p.h+stackY
		case "store":
			p.h += 3*lid + len(rows)*(cmpH+cmpGap)
			for _, l := range rows {
				p.natural = max(p.natural, len(l.runs)*cmpWidth(l)+(len(l.runs)-1)*cmpGap+2*cmpInset)
			}
		}
		all = append(all, p)
		byItem[it] = p
		if it.kind == "group" {
			groups = append(groups, p)
		} else {
			parts = append(parts, p)
		}
	}

	span := func(p *placed) rect { return rect{p.col0, p.row0, p.col1, p.row1} }
	meets := func(a, b rect) bool { return a.x0 <= b.x1 && b.x0 <= a.x1 && a.y0 <= b.y1 && b.y0 <= a.y1 }
	holds := func(a, b rect) bool { return a.x0 <= b.x0 && b.x1 <= a.x1 && a.y0 <= b.y0 && b.y1 <= a.y1 }
	for i, p := range all {
		for _, q := range all[:i] {
			sp, sq := span(p), span(q)
			pg, qg := p.kind == "group", q.kind == "group"
			if meets(sp, sq) && (pg == qg || pg && !holds(sp, sq) || qg && !holds(sq, sp)) {
				r.fail(p.line, fmt.Errorf("%w %s %q", ErrOverlap, q.kind, q.name))
				break
			}
		}
	}
	members := map[*placed][]*placed{}
	for _, g := range groups {
		for _, p := range parts {
			if holds(span(g), span(p)) {
				members[g] = append(members[g], p)
			}
		}
		if len(members[g]) == 0 {
			r.fail(g.line, fmt.Errorf("%w: group %q holds no part in its cells", ErrUnknownBox, g.name))
		}
	}

	var conns []*conn
	named := func(t text) *placed {
		it, ok := a.named[t.s]
		if !ok {
			r.fail(t.line, fmt.Errorf("%w %q", ErrUnknownBox, t.s))
			return nil
		}
		return byItem[it]
	}
	railOut := map[*placed]bool{}
	for _, ar := range a.arrows {
		from, to := named(ar.from), named(ar.to)
		if ar.from.s == ar.to.s {
			r.fail(ar.line, fmt.Errorf("%w: an arrow joins two parts, not %q to itself", ErrFields, ar.from.s))
		}
		if ar.actor.s != "" {
			r.actor(ar.actor)
		}
		var v *valve
		if ar.via.s != "" {
			if v = a.valves[ar.via.s]; v == nil {
				r.fail(ar.line, fmt.Errorf("%w %q: via names a valve", ErrUnknownBox, ar.via.s))
			}
		}
		if from != nil && from.kind == "rail" {
			if railOut[from] {
				r.fail(ar.line, fmt.Errorf("%w: rail %q has one connection out, from its lower end", ErrFields, from.name))
			}
			railOut[from] = true
		}
		if from != nil && to != nil && from != to {
			conns = append(conns, &conn{a: ar, from: from, to: to, via: v})
		}
	}
	for _, v := range a.valves {
		r.actor(v.lane)
		r.sline(v.marks, "ts", v.lane.s)
	}
	if len(r.errs) > before {
		return
	}

	bundle := func() {
		for i, c := range conns {
			c.bundle = c
			for _, o := range conns[:i] {
				if o.from == c.from && o.out == c.out && o.a.style == c.a.style && o.a.actor.s == c.a.actor.s && c.from.kind != "rail" {
					c.bundle = o.bundle
					break
				}
			}
		}
	}
	for _, c := range conns {
		c.out, c.in = sides(c.from, c.to)
	}
	bundle()
	ports := map[*placed][4]int{}
	for _, c := range conns {
		n, m := ports[c.from], ports[c.to]
		if c.bundle == c {
			n[c.out]++
		}
		ports[c.from] = n
		m[c.in]++
		ports[c.to] = m
	}
	for _, p := range parts {
		if p.kind == "rail" {
			continue
		}
		n := ports[p]
		p.h = max(p.h, (max(n[east], n[west])+1)*track)
		p.natural = max(p.natural, (max(n[north], n[south])+1)*track)
	}

	nc, nr := 0, 0
	for _, p := range all {
		nc, nr = max(nc, p.col1), max(nr, p.row1)
	}
	cols, rows := make([]int, nc), make([]int, nr)
	colFilled, rowFilled := make([]bool, nc), make([]bool, nr)
	for _, p := range parts {
		if p.kind == "rail" {
			if p.natural > 0 {
				cols[p.col0-1] = max(cols[p.col0-1], p.natural)
				colFilled[p.col0-1] = true
			}
			continue
		}
		if p.col0 == p.col1 {
			cols[p.col0-1] = max(cols[p.col0-1], p.natural)
			colFilled[p.col0-1] = true
		}
		if p.row0 == p.row1 {
			rows[p.row0-1] = max(rows[p.row0-1], p.h)
			rowFilled[p.row0-1] = true
		}
	}
	for i := range cols {
		if !colFilled[i] {
			cols[i] = rail
		}
	}
	for i := range rows {
		if !rowFilled[i] {
			rows[i] = rail
		}
	}
	across := func(sizes []int, first, last int) int {
		have := cellGap * (last - first)
		for _, s := range sizes[first-1 : last] {
			have += s
		}
		return have
	}
	spans := slices.Clone(all)
	slices.SortStableFunc(spans, func(a, b *placed) int { return (a.col1 - a.col0) - (b.col1 - b.col0) })
	for _, p := range spans {
		if have := across(cols, p.col0, p.col1); (p.col0 != p.col1 || p.kind == "group") && p.natural > have {
			widen(cols, colFilled, p.col0-1, p.col1-1, p.natural-have)
		}
	}
	slices.SortStableFunc(spans, func(a, b *placed) int { return (a.row1 - a.row0) - (b.row1 - b.row0) })
	for _, p := range spans {
		if have := across(rows, p.row0, p.row1); p.kind != "group" && p.row0 != p.row1 && p.h > have {
			widen(rows, rowFilled, p.row0-1, p.row1-1, p.h-have)
		}
	}

	total := 2*edge + across(cols, 1, nc)
	if total > systemWidth {
		widths := make([]string, nc)
		for i, c := range cols {
			widths[i] = fmt.Sprintf("column %c %d", 'A'+i, c)
		}
		r.fail(a.line, fmt.Errorf("%w: the drawing needs %dpx, its column has %d: %s", ErrTooWide, total, systemWidth, strings.Join(widths, ", ")))
		return
	}
	widen(cols, colFilled, 0, nc-1, systemWidth-total)
	colX, rowY := make([]int, nc), make([]int, nr)
	for i := range cols {
		colX[i] = edge
		if i > 0 {
			colX[i] = colX[i-1] + cols[i-1] + cellGap
		}
	}
	for i := range rows {
		rowY[i] = edge
		if i > 0 {
			rowY[i] = rowY[i-1] + rows[i-1] + cellGap
		}
	}
	wide, tall := systemWidth, rowY[nr-1]+rows[nr-1]+edge

	for _, p := range parts {
		x0, x1 := colX[p.col0-1], colX[p.col1-1]+cols[p.col1-1]
		y0, y1 := rowY[p.row0-1], rowY[p.row1-1]+rows[p.row1-1]
		switch {
		case p.kind == "rail":
			p.x, p.w, p.y, p.h = (x0+x1)/2, 0, y0, y1-y0
		case p.kind == "person":
			p.x, p.w, p.y = (x0+x1-p.natural)/2, p.natural, (y0+y1-p.h)/2
		case p.row0 != p.row1:
			p.x, p.w, p.y, p.h = x0, x1-x0, y0, y1-y0
		default:
			p.x, p.w, p.y = x0, x1-x0, y0
		}
	}
	tabs := map[*placed]rect{}
	for _, g := range groups {
		ms := members[g]
		u := ms[0].rect()
		for _, m := range ms[1:] {
			b := m.rect()
			u = rect{min(u.x0, b.x0), min(u.y0, b.y0), max(u.x1, b.x1), max(u.y1, b.y1)}
		}
		g.x, g.y = u.x0-groupPad, u.y0-groupPad
		g.w, g.h = u.x1-u.x0+2*groupPad, u.y1-u.y0+2*groupPad
		tw := int(math.Ceil(g.lines[0].width())) + 12
		g.w = max(g.w, tabInset+tw+10)
		tabs[g] = rect{g.x + tabInset, g.y - 8, g.x + tabInset + tw, g.y + 8}
	}

	for _, c := range conns {
		c.out, c.in = sides(c.from, c.to)
	}
	bundle()

	covers := slices.Collect(maps.Values(tabs))
	var railLabels []rect
	for _, p := range parts {
		if p.kind == "rail" {
			l := p.lines[0]
			railLabels = append(railLabels, rect{p.x + 4, p.y, p.x + 6 + int(math.Ceil(l.width())) + padRight, p.y + 16})
		}
	}
	covers = append(covers, railLabels...)
	pitch := 2*balloonR + 4
	nth := func(co *callout) int {
		n := 0
		for _, o := range callouts {
			if o == co {
				return n
			}
			if o.p == co.p && o.i == co.i {
				n++
			}
		}
		return n
	}
	for _, co := range callouts {
		if !co.corner {
			continue
		}
		co.x, co.y = co.p.x+co.p.w-nth(co)*pitch, co.p.y
		if t, ok := tabs[co.p]; ok {
			co.x = t.x1 + 4 + balloonR + nth(co)*pitch
		} else if co.p.kind == "store" {
			co.y += lid
		}
		covers = append(covers, co.rect())
	}
	for _, co := range callouts {
		if co.corner {
			continue
		}
		p := co.p
		x0, base := r.lineAt(p, co.i)
		_, end := p.lines[co.i].reach(float64(x0))
		n := map[int]int{}
		for _, c := range conns {
			if c.from == p && c.bundle == c {
				n[c.out]++
			}
			if c.to == p {
				n[c.in]++
			}
		}
		right, left := p.x+p.w+8, p.x-8
		for _, g := range groups {
			if slices.Contains(members[g], p) {
				if gx := g.x + g.w; gx-p.x-p.w < 8+2*balloonR {
					right = max(right, gx+4)
				}
				if p.x-g.x < 8+2*balloonR {
					left = min(left, g.x-4)
				}
			}
		}
		co.y = base - 4
		if n[east] > n[west] {
			co.x = left - balloonR - nth(co)*pitch
			co.dx = x0 - 2 - co.x
		} else {
			co.x = right + balloonR + nth(co)*pitch
			co.dx = int(math.Round(end)) + 2 - co.x
		}
		covers = append(covers, co.rect())
	}

	type end struct {
		c    *conn
		near *placed
		side int
	}
	var keys []end
	ends := map[end][]end{}
	add := func(c *conn, near *placed, side int) {
		k := end{nil, near, side}
		if _, ok := ends[k]; !ok {
			keys = append(keys, k)
		}
		ends[k] = append(ends[k], end{c, near, side})
	}
	for _, c := range conns {
		if c.bundle == c {
			add(c, c.from, c.out)
		}
		add(c, c.to, c.in)
	}
	along := func(p *placed, side int) (int, int) {
		switch {
		case p.kind == "rail" && side == east:
			return p.y + 3*track, p.y + p.h
		case p.kind == "rail" && side == west:
			return p.y, p.y + p.h
		case p.kind == "rail":
			return p.x, p.x
		case side == east || side == west:
			return p.y, p.y + p.h
		case p.kind == "store":
			return p.x + p.w/4, p.x + p.w*3/4
		case p.kind == "person" && side == north:
			return p.x + p.w/2 - track, p.x + p.w/2 + track
		}
		return p.x, p.x + p.w
	}
	clearOfTabs := func(p *placed, side, lo, hi int) (int, int) {
		var spans [][2]int
		for _, t := range covers {
			t = inflate(t, clear)
			var in bool
			switch side {
			case north:
				in = t.y0 < p.y && p.y-cellGap < t.y1
			case south:
				in = t.y0 < p.y+p.h+cellGap && p.y+p.h < t.y1
			case west:
				in = t.x0 < p.x && p.x-cellGap < t.x1
			case east:
				in = t.x0 < p.x+p.w+cellGap && p.x+p.w < t.x1
			}
			if in && (side == north || side == south) {
				spans = append(spans, [2]int{t.x0, t.x1})
			} else if in {
				spans = append(spans, [2]int{t.y0, t.y1})
			}
		}
		if l, h, ok := open(lo, hi, spans); ok {
			return l, h
		}
		return lo, hi
	}
	rank := func(k end) int {
		es := ends[k]
		switch {
		case k.near.kind == "rail":
			return 0
		case len(es) == 1 && es[0].c.from == k.near && slices.ContainsFunc(conns, func(o *conn) bool { return o != es[0].c && o.bundle == es[0].c }):
			return 2
		}
		return min(len(es), 2)
	}
	slices.SortStableFunc(keys, func(a, b end) int { return rank(b) - rank(a) })
	placedAt := map[*point]bool{}
	for _, k := range keys {
		es := ends[k]
		far := func(e end) (int, int, int) {
			if e.c.from != k.near {
				lo, hi := along(e.c.from, k.side)
				return lo, hi, 1
			}
			lo, hi, n := math.MaxInt, math.MinInt, 0
			for _, o := range conns {
				if o.bundle == e.c.bundle {
					olo, ohi := along(o.to, k.side)
					lo, hi, n = min(lo, olo), max(hi, ohi), n+1
				}
			}
			return lo, hi, n
		}
		slices.SortStableFunc(es, func(a, b end) int {
			alo, ahi, _ := far(a)
			blo, bhi, _ := far(b)
			return (alo + ahi) - (blo + bhi)
		})
		lo, hi := along(k.near, k.side)
		if k.near.kind != "rail" {
			lo, hi = clearOfTabs(k.near, k.side, lo, hi)
		}
		facing := func(e end) (int, int, bool) {
			fp, fs := e.c.from, e.c.out
			if e.c.from == k.near {
				fp, fs = e.c.to, e.c.in
			}
			alone := e.c.bundle == e.c && !slices.ContainsFunc(conns, func(o *conn) bool { return o != e.c && o.bundle == e.c })
			if !alone || fs != (k.side+2)%4 || fp.kind == "rail" || len(ends[end{nil, fp, fs}]) != 1 {
				return 0, 0, false
			}
			flo, fhi := along(fp, fs)
			flo, fhi = clearOfTabs(fp, fs, flo, fhi)
			return flo + track, fhi - track, true
		}
		vs, pinned := make([]int, len(es)), make([]bool, len(es))
		if len(es) > 1 && k.near.kind != "rail" {
			prev, since := lo, 0
			for i, e := range es {
				even := lo + (hi-lo)*(i+1)/(len(es)+1)
				a, b, ok := facing(e)
				a, b = max(a, prev+(since+1)*track), min(b, hi-(len(es)-i)*track)
				if ok && a <= b {
					vs[i], pinned[i], prev, since = min(max(even, a), b), true, min(max(even, a), b), 0
				} else {
					since++
				}
			}
			for i := 0; i < len(es); {
				j := i
				for j < len(es) && !pinned[j] {
					j++
				}
				from, to := lo, hi
				if i > 0 {
					from = vs[i-1]
				}
				if j < len(es) {
					to = vs[j]
				}
				for m := i; m < j; m++ {
					vs[m] = from + (to-from)*(m-i+1)/(j-i+1)
				}
				i = j + 1
			}
		}
		for i, e := range es {
			near, other := &e.c.q, &e.c.bundle.p
			if e.c.from == k.near && e.c.out == k.side {
				near, other = &e.c.p, &e.c.q
			}
			v := lo + (hi-lo)*(i+1)/(len(es)+1)
			fv := other.x
			if k.side == east || k.side == west {
				fv = other.y
			}
			flo, fhi, n := far(e)
			switch {
			case e.c.from.kind == "rail" && (k.side == east || k.side == west):
				v = max(min(hi, fhi)-2*track, lo, flo)
			case k.near.kind == "rail" && placedAt[other]:
				v = min(max(fv, lo), hi)
			case k.near.kind == "rail":
				v = min(max((flo+fhi)/2, lo), hi)
			case len(es) > 1:
				v = vs[i]
			case n == 1 && placedAt[other] && lo+track <= fv && fv <= hi-track:
				v = fv
			case !placedAt[other] && max(lo, flo)+track < min(hi, fhi)-track:
				v = (max(lo, flo) + min(hi, fhi)) / 2
			}
			*near = map[int]point{east: {k.near.x + k.near.w, v}, west: {k.near.x, v}, south: {v, k.near.y + k.near.h}, north: {v, k.near.y}}[k.side]
			placedAt[near] = true
		}
	}
	for _, c := range conns {
		c.p = c.bundle.p
	}

	var xs, ys []int
	gaps := func(starts, sizes []int, whole int) []int {
		var vs []int
		lo := 0
		for i := range starts {
			for v := lo + track; v <= starts[i]-track; v += track {
				vs = append(vs, v)
			}
			lo = starts[i] + sizes[i]
		}
		for v := lo + track; v <= whole-track; v += track {
			vs = append(vs, v)
		}
		return vs
	}
	xs, ys = gaps(colX, cols, wide), gaps(rowY, rows, tall)
	for _, c := range conns {
		xs, ys = append(xs, c.p.x, c.q.x), append(ys, c.p.y, c.q.y)
	}
	slices.Sort(xs)
	slices.Sort(ys)
	xs, ys = slices.Compact(xs), slices.Compact(ys)

	var taken []seg
	var takenBy []*conn
	var ruled []seg
	var heads []seg
	labels := railLabels
	turns := map[point]*conn{}
	for _, c := range conns {
		turns[c.p], turns[c.q] = c.bundle, c
	}
	for _, g := range groups {
		a, b, c, d := point{g.x, g.y}, point{g.x + g.w, g.y}, point{g.x + g.w, g.y + g.h}, point{g.x, g.y + g.h}
		ruled = append(ruled, segOf(a, b), segOf(b, c), segOf(d, c), segOf(a, d))
	}
	byLength := slices.Clone(conns)
	slices.SortStableFunc(byLength, func(a, b *conn) int {
		return abs(a.p.x-a.q.x) + abs(a.p.y-a.q.y) - abs(b.p.x-b.q.x) - abs(b.p.y-b.q.y)
	})
	for _, c := range byLength {
		var walls []rect
		for _, p := range all {
			switch {
			case p == c.from || p == c.to:
				walls = append(walls, p.rect())
			case p.kind != "group":
				walls = append(walls, inflate(p.rect(), clear))
			}
		}
		for _, g := range groups {
			walls = append(walls, inflate(tabs[g], 2))
		}
		for _, co := range callouts {
			walls = append(walls, inflate(co.rect(), 2))
		}
		for _, l := range labels {
			walls = append(walls, inflate(l, 2))
		}
		shared := func(s seg) bool {
			for i, t := range taken {
				if takenBy[i] == c.bundle && t.vertical() == s.vertical() && (s.vertical() && t.a.x == s.a.x && t.a.y <= s.a.y && s.b.y <= t.b.y || !s.vertical() && t.a.y == s.a.y && t.a.x <= s.a.x && s.b.x <= t.b.x) {
					return true
				}
			}
			return false
		}
		class, _ := arrowStyle(cmp.Or(c.a.actor.s, c.from.lane.s), c.a.style)
		if c.from.kind == "rail" {
			class = "w heavy"
		}
		in := (c.in + 2) % 4
		room0, room1 := 0, 0
		if c.a.style == "dotted" {
			room0 = room(class) + hop
		}
		if c.to.kind != "rail" {
			room1 = room(class) + 2 + hop
		}
		own := []seg{
			segOf(c.p, point{c.p.x + step[c.out].x*room0, c.p.y + step[c.out].y*room0}),
			segOf(point{c.q.x - step[in].x*room1, c.q.y - step[in].y*room1}, c.q),
		}
		strict := true
		free := func(a, b point) bool {
			s := segOf(a, b)
			for _, z := range heads {
				if z.vertical() != s.vertical() && inside(b, z) {
					return false
				}
			}
			for _, z := range own {
				if strict && inside(b, z) && slices.ContainsFunc(taken, func(t seg) bool { return t.vertical() != z.vertical() && inside(b, t) }) {
					return false
				}
			}
			for _, o := range walls {
				if s.vertical() && o.x0 < s.a.x && s.a.x < o.x1 && s.a.y < o.y1 && o.y0 < s.b.y ||
					!s.vertical() && o.y0 < s.a.y && s.a.y < o.y1 && s.a.x < o.x1 && o.x0 < s.b.x {
					return false
				}
			}
			on := shared(s)
			for i, t := range taken {
				if t.vertical() == s.vertical() && alongside(s, t) && (!on || takenBy[i] != c.bundle) {
					return false
				}
			}
			for _, t := range ruled {
				if t.vertical() == s.vertical() && alongside(s, t) {
					return false
				}
			}
			return b == c.q || turns[b] == nil || turns[b] == c.bundle && on
		}
		extra := func(a, b point) int {
			s := segOf(a, b)
			if shared(s) {
				return -abs(b.x-a.x) - abs(b.y-a.y)
			}
			n := 0
			for i, t := range taken {
				if takenBy[i] != c.bundle && t.vertical() != s.vertical() && (s.vertical() && t.a.y == b.y && t.a.x < b.x && b.x < t.b.x || !s.vertical() && t.a.x == b.x && t.a.y < b.y && b.y < t.b.y) {
					n++
				}
			}
			return crossCost * n
		}
		way, ok := search(xs, ys, c.p, c.q, c.out, in, room0, room1, free, extra)
		strict = false
		if loose, lok := search(xs, ys, c.p, c.q, c.out, in, 0, 0, free, extra); !ok || lok && len(loose) < len(way) {
			way, ok = loose, lok
		}
		if !ok {
			r.fail(c.a.line, fmt.Errorf("%w from %q to %q: a part stands in every way between the sides that face each other; move a part or leave a cell empty between them", ErrNoWay, c.a.from.s, c.a.to.s))
			continue
		}
		c.way = way
		heads = append(heads, own...)
		c.start = way[0]
		for c.from0 = 0; c.from0+1 < len(way); c.from0++ {
			a, b := way[c.from0], way[c.from0+1]
			d := point{sign(b.x - a.x), sign(b.y - a.y)}
			ahead := func(p point) int { return (p.x-a.x)*d.x + (p.y-a.y)*d.y }
			reach := a
			for grown := true; grown && reach != b; {
				grown = false
				for i, t := range taken {
					if takenBy[i] != c.bundle || t.vertical() != (d.x == 0) || !lies(reach, t) {
						continue
					}
					far := t.b
					if d.x < 0 || d.y < 0 {
						far = t.a
					}
					if ahead(far) > ahead(reach) {
						reach, grown = far, true
					}
				}
				if ahead(reach) > ahead(b) {
					reach = b
				}
			}
			c.start = reach
			if reach != b {
				break
			}
		}
		for i := 1; i < len(way); i++ {
			taken = append(taken, segOf(way[i-1], way[i]))
			takenBy = append(takenBy, c.bundle)
		}
		for _, pt := range way {
			if turns[pt] == nil {
				turns[pt] = c.bundle
			}
		}
		if len(c.a.label) > 0 {
			c.label = r.label(c, rect{0, 0, wide, tall}, all, callouts, taken, labels)
			labels = append(labels, c.label.rect)
		}
	}
	if len(r.errs) > before {
		return
	}

	r.w("<svg class=\"assembly\" width=\"%d\" height=\"%d\" viewBox=\"0 0 %d %d\">\n", wide, tall, wide, tall)
	for _, g := range groups {
		r.w("  <rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" class=\"%s\"/>\n", g.x, g.y, g.w, g.h, strings.TrimSpace("grp "+g.lane.s))
		t := tabs[g]
		r.w("  <rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"16\" class=\"tab\"/>\n", t.x0, t.y0, t.x1-t.x0)
		r.line(g.lines[0], t.x0+6-padLeft, t.x1-t.x0-12+padLeft+padRight, g.y+4)
	}
	for _, p := range parts {
		switch {
		case p.kind != "rail":
			r.part(p)
		case !railOut[p]:
			r.rail(p, nil, conns)
		}
	}
	last := map[*valve]point{}
	for _, c := range conns {
		actor := cmp.Or(c.a.actor.s, c.from.lane.s)
		class, marker := arrowStyle(actor, c.a.style)
		if c.a.style == "dotted" {
			head := strings.TrimSuffix("a-"+actor, "-")
			marker = fmt.Sprintf(` marker-start="url(#%s)" marker-end="url(#%s)"`, head, head)
		}
		switch {
		case c.from.kind == "rail":
			r.rail(c.from, c, conns)
		case c.to.kind == "rail":
			r.w("  <path d=\"%s\" class=\"%s\"/>", hopped(c, conns), class)
			r.w("<circle cx=\"%d\" cy=\"%d\" r=\"4.5\" class=\"dot %s\"/>\n", c.q.x, c.q.y, c.to.lane.s)
		default:
			d := hopped(c, conns)
			r.arrive(c.a.line, d, class, c.a.style == "dotted", c.a.from.s, c.a.to.s)
			r.w("  <path d=\"%s\" class=\"%s\"%s/>\n", d, class, marker)
		}
		if j := c.start; j != c.way[0] {
			r.w("  <circle cx=\"%d\" cy=\"%d\" r=\"3.5\" class=\"dot %s\"/>\n", j.x, j.y, actor)
		}
		if c.via != nil {
			a, b := c.way[len(c.way)-2], c.way[len(c.way)-1]
			d := step[(c.in+2)%4]
			back := min(26, (abs(b.x-a.x)+abs(b.y-a.y))/2)
			v := point{b.x - d.x*back, b.y - d.y*back}
			u, n := point{d.x * 8, d.y * 8}, point{-d.y * 7, d.x * 7}
			r.w("  <path d=\"M%d,%d L%d,%d L%d,%d Z M%d,%d L%d,%d L%d,%d Z\" class=\"valve %s\"/>\n",
				v.x-u.x+n.x, v.y-u.y+n.y, v.x, v.y, v.x-u.x-n.x, v.y-u.y-n.y,
				v.x+u.x+n.x, v.y+u.y+n.y, v.x, v.y, v.x+u.x-n.x, v.y+u.y-n.y, c.via.lane.s)
			if f, ok := last[c.via]; !ok || v.y > f.y {
				last[c.via] = v
			}
		}
		if c.label != nil {
			for i, l := range c.label.lines {
				r.line(l, c.label.x0-padLeft, c.label.x1-c.label.x0+padLeft+padRight, c.label.base+i*labelPitch)
			}
		}
	}
	for _, v := range slices.SortedFunc(maps.Keys(last), func(a, b *valve) int { return a.line - b.line }) {
		if v.marks.s != "" {
			l := r.sline(v.marks, "ts", v.lane.s)
			f := last[v]
			r.line(l, f.x-int(l.width())/2-padLeft, 0, f.y+balloonR+12+4)
		}
	}
	for _, co := range callouts {
		r.w("  %s\n", r.calloutSVG(co))
	}
	r.w("</svg>\n")
}

type callout struct {
	m      drawnMark
	p      *placed
	i      int
	corner bool
	x, y   int
	dx     int
}

func (co *callout) rect() rect {
	return rect{co.x - balloonR, co.y - balloonR, co.x + balloonR, co.y + balloonR}
}

func (r *renderer) calloutSVG(co *callout) string {
	leader := ""
	if co.dx != 0 {
		leader = fmt.Sprintf(`<line x1="0" y1="0" x2="%d" y2="0"/>`, co.dx)
	}
	return fmt.Sprintf(`<g class="%s" data-item="%s" transform="translate(%d,%d)">%s<circle r="%d"/><text y="4">%s</text></g>`,
		strings.TrimSpace("bal "+co.m.actor), co.m.n, co.x, co.y, leader, balloonR, co.m.n)
}

type label struct {
	rect
	lines []sline
	base  int
}

func (r *renderer) label(c *conn, bounds rect, all []*placed, callouts []*callout, taken []seg, labels []rect) *label {
	lb := &label{}
	w := 0
	for _, t := range c.a.label {
		l := r.sline(t, "ts", "")
		lb.lines = append(lb.lines, l)
		w = max(w, int(math.Ceil(l.width())))
	}
	h := labelPitch*(len(lb.lines)-1) + 12
	drawn := append([]point{c.start}, c.way[c.from0+1:]...)
	var stretches []seg
	for i := 1; i < len(drawn); i++ {
		stretches = append(stretches, segOf(drawn[i-1], drawn[i]))
	}
	length := func(s seg) int { return s.b.x - s.a.x + s.b.y - s.a.y }
	slices.SortStableFunc(stretches, func(a, b seg) int { return length(b) - length(a) })
	var blocks []rect
	for _, p := range all {
		if p.kind != "group" {
			blocks = append(blocks, p.rect())
		}
	}
	for _, co := range callouts {
		blocks = append(blocks, co.rect())
	}
	blocks = append(blocks, labels...)
	meets := func(o rect) bool {
		if o.x0 < bounds.x0 || o.y0 < bounds.y0 || o.x1 > bounds.x1 || o.y1 > bounds.y1 {
			return true
		}
		for _, b := range blocks {
			if o.x0 < b.x1 && b.x0 < o.x1 && o.y0 < b.y1 && b.y0 < o.y1 {
				return true
			}
		}
		for _, p := range all {
			if p.kind == "group" {
				g := p.rect()
				inside := g.x0 < o.x0 && o.x1 < g.x1 && g.y0 < o.y0 && o.y1 < g.y1
				apart := o.x1 < g.x0 || g.x1 < o.x0 || o.y1 < g.y0 || g.y1 < o.y0
				if !inside && !apart {
					return true
				}
			}
		}
		for _, s := range taken {
			if s.a.x <= o.x1 && o.x0 <= s.b.x && s.a.y <= o.y1 && o.y0 <= s.b.y {
				return true
			}
		}
		return false
	}
	var tried []rect
	for _, s := range stretches {
		var at []rect
		if s.vertical() {
			y0 := (s.a.y+s.b.y)/2 - h/2
			at = []rect{{s.a.x + 8, y0, s.a.x + 8 + w, y0 + h}, {s.a.x - 8 - w, y0, s.a.x - 8, y0 + h}}
		} else {
			x0 := s.a.x + 10
			at = []rect{{x0, s.a.y - 5 - h, x0 + w, s.a.y - 5}, {x0, s.a.y + 5, x0 + w, s.a.y + 5 + h}}
		}
		for _, o := range at {
			tried = append(tried, o)
			if !meets(inflate(o, 2)) {
				lb.rect, lb.base = o, o.y0+10
				return lb
			}
		}
	}
	lb.rect = tried[0]
	lb.base = lb.y0 + 10
	return lb
}

func (r *renderer) rail(p *placed, out *conn, conns []*conn) {
	r.line(p.lines[0], p.x+6-padLeft, 0, p.y+12)
	class := "w heavy " + p.lane.s
	if out == nil {
		r.w("  <path d=\"M%d,%d V%d\" class=\"%s\"/>\n", p.x, p.y, p.y+p.h, class)
		return
	}
	way := hopped(out, conns)
	_, rest, _ := strings.Cut(way, " ")
	turn := step[out.out]
	d := fmt.Sprintf("M%d,%d V%d Q%d,%d %d,%d %s", p.x, p.y, out.p.y-railTurn, p.x, out.p.y, p.x+turn.x*railTurn, out.p.y, rest)
	r.arrive(out.a.line, d, class, false, out.a.from.s, out.a.to.s)
	r.w("  <path d=\"%s\" class=\"%s\" marker-end=\"url(#a-heavy-%s)\"/>\n", d, class, p.lane.s)
}

func compartments(l sline) bool {
	marks := 0
	for _, rn := range l.runs {
		marks += len(rn.marks)
	}
	return len(l.runs) > 0 && len(l.lead) == 0 && len(l.trail) == marks &&
		!slices.ContainsFunc(l.runs, func(rn run) bool { return rn.kind() != "chip" })
}

func chipsOnly(l sline) sline {
	l.runs = slices.DeleteFunc(slices.Clone(l.runs), func(rn run) bool { return rn.kind() == "" && strings.TrimSpace(rn.s) == "" })
	return l
}

func cmpWidth(l sline) int {
	w := 0.0
	for _, rn := range l.runs {
		w = max(w, textWidth(rn.s, "tc")+float64(len(rn.marks)*(2*balloonR+4)))
	}
	return int(math.Ceil(w)) + 2*padLeft
}

func (r *renderer) lineAt(p *placed, i int) (int, int) {
	x, base := p.x+padLeft, p.y+16
	switch p.kind {
	case "stack":
		x, base = x+stackX, base+stackY
	case "store":
		base += 2 * lid
	case "person":
		base += figure
	}
	for j := 1; j <= i; j++ {
		base += max(p.lines[j-1].size, p.lines[j].size)
	}
	if p.kind == "store" || p.kind == "person" {
		x = p.x + p.w/2 - int(math.Ceil(p.lines[i].width()))/2
	}
	return x, base
}

func sides(a, b *placed) (int, int) {
	across := east
	if b.col1 < a.col0 {
		across = west
	}
	apart := b.col0 > a.col1 || b.col1 < a.col0
	switch {
	case a.kind == "rail" || b.kind == "rail":
	case apart && a.row0 > b.row1:
		return across, south
	case apart && b.row0 > a.row1:
		return south, (across + 2) % 4
	}
	switch {
	case b.col0 > a.col1:
		return east, west
	case b.col1 < a.col0:
		return west, east
	case b.row0 > a.row1:
		return south, north
	case b.row1 < a.row0:
		return north, south
	}
	dx := (b.x + b.w/2) - (a.x + a.w/2)
	dy := (b.y + b.h/2) - (a.y + a.h/2)
	switch {
	case abs(dx) >= abs(dy) && dx >= 0:
		return east, west
	case abs(dx) >= abs(dy):
		return west, east
	case dy >= 0:
		return south, north
	}
	return north, south
}

func abs(v int) int { return max(v, -v) }

func sign(v int) int { return min(max(v, -1), 1) }

func lies(p point, s seg) bool {
	return s.a.x <= p.x && p.x <= s.b.x && s.a.y <= p.y && p.y <= s.b.y
}

func inside(p point, s seg) bool {
	return lies(p, s) && p != s.a && p != s.b
}

func inflate(o rect, by int) rect { return rect{o.x0 - by, o.y0 - by, o.x1 + by, o.y1 + by} }

func alongside(s, t seg) bool {
	if s.vertical() {
		return abs(s.a.x-t.a.x) < track/2 && s.a.y < t.b.y && t.a.y < s.b.y
	}
	return abs(s.a.y-t.a.y) < track/2 && s.a.x < t.b.x && t.a.x < s.b.x
}

func (r *renderer) part(p *placed) {
	class := "bx " + p.lane.s
	if p.planned {
		class += " ph"
	}
	x, y, w, h := p.x, p.y, p.w, p.h
	switch p.kind {
	case "store":
		r.w("  <path d=\"M%d,%d V%d A%d,%d 0 0,0 %d,%d V%d Z\" class=\"%s\"/>", x, y+lid, y+h-lid, w/2, lid, x+w, y+h-lid, y+lid, class)
		r.w("<ellipse cx=\"%d\" cy=\"%d\" rx=\"%d\" ry=\"%d\" class=\"%s\"/>\n", x+w/2, y+lid, w/2, lid, class)
		for i, l := range p.lines {
			lx, base := r.lineAt(p, i)
			r.line(l, lx-padLeft, int(math.Ceil(l.width()))+padLeft+padRight, base)
		}
		ry := y + h - lid - 4 - len(p.rows)*(cmpH+cmpGap) + cmpGap
		for _, l := range p.rows {
			n := len(l.runs)
			cw := (w - 2*cmpInset - (n-1)*cmpGap) / n
			for j, rn := range l.runs {
				cx := x + cmpInset + j*(cw+cmpGap)
				r.w("  <rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" class=\"bx cmp\"/>", cx, ry, cw, cmpH)
				r.w("<text x=\"%d\" y=\"%d\" class=\"tc\">%s</text>", cx+padLeft, ry+cmpH/2+5, esc(rn.s))
				for k, m := range slices.Backward(rn.marks) {
					r.w("%s", r.markSVG(m, cx+cw-3-balloonR-k*(2*balloonR+4), ry+cmpH/2))
				}
				r.w("\n")
			}
			ry += cmpH + cmpGap
		}
	case "person":
		cx := x + w/2
		r.w("  <circle cx=\"%d\" cy=\"%d\" r=\"15\" class=\"%s\"/>", cx, y+16, class)
		r.w("<path d=\"M%d,%d C%d,%d %d,%d %d,%d C%d,%d %d,%d %d,%d Z\" class=\"%s\"/>\n",
			cx-30, y+figure-1, cx-30, y+45, cx-16, y+37, cx, y+37, cx+16, y+37, cx+30, y+45, cx+30, y+figure-1, class)
		r.lines(p.lines, y+figure+16, func(l sline) (int, int) {
			lw := int(math.Ceil(l.width()))
			return cx - lw/2 - padLeft, lw + padLeft + padRight
		})
	default:
		r.item(p)
	}
}

func hopped(c *conn, conns []*conn) string {
	way := append([]point{c.start}, c.way[c.from0+1:]...)
	last := len(way) - 1
	in := (c.in + 2) % 4
	if c.to.kind != "rail" {
		way[last] = point{way[last].x - 2*step[in].x, way[last].y - 2*step[in].y}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "M%d,%d", way[0].x, way[0].y)
	for i := 1; i < len(way); i++ {
		a, z := way[i-1], way[i]
		if a.x == z.x {
			fmt.Fprintf(&b, " V%d", z.y)
			continue
		}
		dir := 1
		if z.x < a.x {
			dir = -1
		}
		var at []int
		for _, o := range conns {
			if o == c {
				continue
			}
			ow := append([]point{o.start}, o.way[o.from0+1:]...)
			for j := 1; j < len(ow); j++ {
				t := segOf(ow[j-1], ow[j])
				if t.vertical() && t.a.y < a.y && a.y < t.b.y && min(a.x, z.x) < t.a.x && t.a.x < max(a.x, z.x) {
					at = append(at, t.a.x)
				}
			}
		}
		slices.SortFunc(at, func(u, v int) int { return (u - v) * dir })
		at = slices.Compact(at)
		sweep := map[int]int{1: 1, -1: 0}[dir]
		for _, x := range at {
			fmt.Fprintf(&b, " H%d A%d,%d 0 0,%d %d,%d", x-dir*hop, hop, hop, sweep, x+dir*hop, a.y)
		}
		fmt.Fprintf(&b, " H%d", z.x)
	}
	return b.String()
}

func search(xs, ys []int, p, q point, out, in, room0, room1 int, free func(a, b point) bool, extra func(a, b point) int) ([]point, bool) {
	id := func(pt point) int {
		i, _ := slices.BinarySearch(xs, pt.x)
		j, _ := slices.BinarySearch(ys, pt.y)
		return i*len(ys) + j
	}
	at := func(node int) point { return point{xs[node/len(ys)], ys[node%len(ys)]} }
	start, goal := id(p), id(q)
	near := func(pt, from point, d, room int) bool {
		u := step[d]
		k := (pt.x-from.x)*u.x + (pt.y-from.y)*u.y
		return 0 < k && k < room && pt == point{from.x + u.x*k, from.y + u.y*k}
	}
	runs := func(pt point) int {
		switch {
		case near(pt, p, out, room0):
			return out
		case near(pt, q, (in+2)%4, room1):
			return in
		}
		return -1
	}
	best := make([]int, 4*len(xs)*len(ys))
	prev := make([]int, len(best))
	for i := range best {
		best[i], prev[i] = math.MaxInt, -1
	}
	best[4*start+out] = 0
	h := &frontier{{0, 4*start + out}}
	for h.Len() > 0 {
		c := heap.Pop(h).(visit)
		if c.cost > best[c.state] {
			continue
		}
		node, d := c.state/4, c.state%4
		if node == goal {
			var way []point
			for s := c.state; s >= 0; s = prev[s] {
				pt := at(s / 4)
				if n := len(way); n >= 2 && (way[n-1].x == pt.x && way[n-2].x == pt.x || way[n-1].y == pt.y && way[n-2].y == pt.y) {
					way[n-1] = pt
					continue
				}
				way = append(way, pt)
			}
			slices.Reverse(way)
			return way, true
		}
		i, j := node/len(ys), node%len(ys)
		for nd := range 4 {
			if nd == (d+2)%4 || node == start && nd != out {
				continue
			}
			ni, nj := i+step[nd].x, j+step[nd].y
			if ni < 0 || nj < 0 || ni >= len(xs) || nj >= len(ys) {
				continue
			}
			next := ni*len(ys) + nj
			if next == start || next == goal && nd != in {
				continue
			}
			if r := runs(at(node)); r >= 0 && nd != r {
				continue
			}
			if r := runs(at(next)); r >= 0 && nd != r {
				continue
			}
			a, b := at(node), at(next)
			if !free(a, b) {
				continue
			}
			cost := c.cost + abs(b.x-a.x) + abs(b.y-a.y) + extra(a, b)
			if nd != d {
				cost += bendCost
			}
			if s := 4*next + nd; cost < best[s] {
				best[s], prev[s] = cost, c.state
				heap.Push(h, visit{cost, s})
			}
		}
	}
	return nil, false
}

type visit struct{ cost, state int }

type frontier []visit

func (f frontier) Len() int           { return len(f) }
func (f frontier) Less(i, j int) bool { return f[i].cost < f[j].cost }
func (f frontier) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }
func (f *frontier) Push(v any)        { *f = append(*f, v.(visit)) }
func (f *frontier) Pop() any {
	old := *f
	v := old[len(old)-1]
	*f = old[:len(old)-1]
	return v
}
