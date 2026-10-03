package onepage

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type text struct {
	s    string
	line int
}

type doc struct {
	title    string
	actors   []actorDecl
	lines    []legendDecl
	marks    []legendDecl
	blocks   []any
	parts    map[string]string
	notes    map[string]bool
	diamonds map[string]bool
	details  map[string]bool
	more     int
}

type actorDecl struct {
	name, colour, label string
	line                int
}

type legendDecl struct {
	kind, actor, label string
	line               int
}

type viewHead struct{ n, title, sub, note text }

type drawing struct{ lines []text }

type titleblock struct{ cells []*cell }

type cell struct {
	kind  string
	label text
	els   []*cellEl
}

type cellEl struct {
	kind string
	text text
	rows [][3]text
}

type details struct{ panels []*panel }

type panel struct {
	notes            bool
	tag, title, note text
	rungs            []*rung
	items            [][3]text
	foot             text
}

type rung struct {
	actor         text
	planned       bool
	title, who, s text
}

type assembly struct {
	line       int
	items      []*item
	named      map[string]*item
	valves     map[string]*valve
	arrows     []*arrow
	title, sub text
	parts      []*part
}

type valve struct {
	name  string
	lane  text
	marks text
	line  int
}

type part struct {
	n, actor        text
	planned         bool
	name, what, why text
}

type spec struct{ cols [][]*howto }

type howto struct {
	no, to, typed text
	content       []any
	more          []text
}

type term struct {
	lines  []text
	beside bool
}

type swimlane struct {
	line   int
	lanes  []*lane
	phases []*phase
	items  []*item
	boxes  map[string]*item
	arrows []*arrow
	frames []*frame
}

type lane struct {
	actor text
	lines []text
}

type phase struct{ n, title text }

type item struct {
	name       string
	kind       string
	planned    bool
	lane       text
	phase      text
	col0, col1 int
	row0, row1 int
	lines      []text
	more       []text
	line       int
}

type arrow struct {
	from, to text
	style    string
	actor    text
	via      text
	label    []text
	line     int
}

type frame struct {
	letter text
	boxes  []text
}

var actorName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type parser struct {
	path   string
	doc    *doc
	errs   []error
	block  any
	opened int
	fence  *fence
	more   *[]text
}

type fence struct {
	kind   string
	beside bool
	line   int
	lines  []text
}

func parse(path string, src []byte) (*doc, []error) {
	p := &parser{path: path, doc: &doc{parts: map[string]string{}, notes: map[string]bool{}, diamonds: map[string]bool{}, details: map[string]bool{}}}
	lines := strings.Split(strings.TrimSuffix(string(src), "\n"), "\n")
	for i, line := range lines {
		n := i + 1
		if p.fence != nil {
			if strings.TrimSpace(line) == "```" {
				p.closeFence()
			} else {
				p.fence.lines = append(p.fence.lines, text{line, n})
			}
			continue
		}
		t := strings.TrimSpace(line)
		switch {
		case t == "" || strings.HasPrefix(t, "#"):
		case strings.HasPrefix(t, "```"):
			p.openFence(strings.Fields(t[3:]), n)
		case t == ":::":
			if p.block == nil {
				p.fail(n, fmt.Errorf("%w: ::: closes no block", ErrUnknownKeyword))
			}
			p.block, p.more = nil, nil
		case strings.HasPrefix(t, ":::"):
			p.more = nil
			p.openBlock(strings.TrimSpace(t[3:]), n)
		default:
			kw, rest, _ := strings.Cut(t, " ")
			p.keyword(kw, strings.TrimSpace(rest), n)
		}
	}
	if p.fence != nil {
		p.fail(p.fence.line, fmt.Errorf("%w: ```%s", ErrUnclosed, p.fence.kind))
	}
	if p.block != nil {
		p.fail(p.opened, fmt.Errorf("%w: a block opened here", ErrUnclosed))
	}
	if p.doc.more != 0 && !slices.ContainsFunc(p.doc.marks, func(m legendDecl) bool { return m.kind == "more" }) {
		p.fail(p.doc.more, ErrUnkeyedMore)
	}
	return p.doc, p.errs
}

func (p *parser) fail(line int, err error) {
	p.errs = append(p.errs, &SourceError{Path: p.path, Line: line, Err: err})
}

func (p *parser) openBlock(name string, n int) {
	if p.block != nil {
		p.fail(n, fmt.Errorf("%w: ::: %s inside another block", ErrUnknownBlock, name))
		return
	}
	var b any
	switch name {
	case "titleblock":
		b = &titleblock{}
	case "details":
		b = &details{}
	case "assembly":
		b = &assembly{line: n, named: map[string]*item{}, valves: map[string]*valve{}}
	case "spec":
		b = &spec{}
	case "swimlane":
		b = &swimlane{line: n, boxes: map[string]*item{}}
	default:
		p.fail(n, fmt.Errorf("%w %q", ErrUnknownBlock, name))
		b = &unknownBlock{}
	}
	p.block, p.opened = b, n
	if _, ok := b.(*unknownBlock); !ok {
		p.doc.blocks = append(p.doc.blocks, b)
	}
}

type unknownBlock struct{}

func (p *parser) openFence(args []string, n int) {
	f := &fence{line: n}
	if len(args) > 0 {
		f.kind = args[0]
	}
	f.beside = len(args) > 1 && args[1] == "beside"
	var ok bool
	switch f.kind {
	case "svg":
		ok = p.block == nil
	case "term":
		s, inSpec := p.block.(*spec)
		ok = inSpec && s.last() != nil
	}
	if _, unknown := p.block.(*unknownBlock); !ok && !unknown {
		p.fail(n, fmt.Errorf("%w: ```%s does not belong here", ErrUnknownBlock, f.kind))
	}
	p.fence = f
}

func (p *parser) closeFence() {
	f := p.fence
	p.fence = nil
	switch b := p.block.(type) {
	case nil:
		if f.kind == "svg" {
			p.doc.blocks = append(p.doc.blocks, &drawing{lines: f.lines})
		}
	case *spec:
		if h := b.last(); h != nil && f.kind == "term" {
			h.content = append(h.content, &term{lines: f.lines, beside: f.beside})
		}
	}
}

func (p *parser) fields(kw, rest string, n, min, want int) ([]text, bool) {
	parts := splitFields(" " + rest)
	if len(parts) < min || len(parts) > want {
		if min == want {
			p.fail(n, fmt.Errorf("%w: %s takes %d, has %d", ErrFields, kw, want, len(parts)))
		} else {
			p.fail(n, fmt.Errorf("%w: %s takes %d to %d, has %d", ErrFields, kw, min, want, len(parts)))
		}
		return nil, false
	}
	f := make([]text, want)
	for i := range f {
		f[i].line = n
		if i < len(parts) {
			f[i].s = strings.TrimSpace(parts[i])
		}
	}
	return f, true
}

func splitFields(s string) []string {
	var parts []string
	start, code := 0, false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\':
			i++
		case s[i] == '`':
			code = !code
		case !code && strings.HasPrefix(s[i:], " | "):
			parts = append(parts, s[start:i])
			start = i + len(" | ")
			i = start - 1
		}
	}
	return append(parts, s[start:])
}

func attrs(f text) (actor text, planned bool, words []string) {
	actor.line = f.line
	for w := range strings.FieldsSeq(f.s) {
		switch {
		case strings.HasPrefix(w, "@"):
			actor.s = w[1:]
		case w == "planned":
			planned = true
		default:
			words = append(words, w)
		}
	}
	return actor, planned, words
}

func (p *parser) keyword(kw, rest string, n int) {
	t := text{rest, n}
	if _, unknown := p.block.(*unknownBlock); unknown {
		return
	}
	if kw == "more" {
		if p.more == nil {
			p.fail(n, fmt.Errorf("%w %q: a detail follows a how-to or a swimlane box", ErrUnknownKeyword, kw))
			return
		}
		*p.more = append(*p.more, t)
		if p.doc.more == 0 {
			p.doc.more = n
		}
		return
	}
	p.more = nil
	switch b := p.block.(type) {
	case nil:
		p.top(kw, rest, n)
	case *unknownBlock:
	case *titleblock:
		p.titleblock(b, kw, t)
	case *details:
		p.details(b, kw, t)
	case *assembly:
		p.assembly(b, kw, t)
	case *spec:
		p.spec(b, kw, t)
	case *swimlane:
		p.swimlane(b, kw, t)
	}
}

func (p *parser) unknown(kw string, n int) {
	p.fail(n, fmt.Errorf("%w %q", ErrUnknownKeyword, kw))
}

func (p *parser) top(kw, rest string, n int) {
	d := p.doc
	switch kw {
	case "title":
		d.title = rest
	case "actor":
		w := strings.Fields(rest)
		if len(w) < 3 || !actorName.MatchString(w[0]) {
			p.fail(n, fmt.Errorf("%w: actor takes a lowercase name, a colour and words", ErrFields))
			return
		}
		if reserved[w[0]] || strings.HasPrefix(w[0], "heavy-") {
			p.fail(n, fmt.Errorf("%w: actor %q", ErrReserved, w[0]))
			return
		}
		if _, ok := palette[w[1]]; !ok {
			p.fail(n, fmt.Errorf("%w %q: the palette has amber, cyan, green and violet", ErrUnknownColour, w[1]))
			return
		}
		d.actors = append(d.actors, actorDecl{w[0], w[1], strings.Join(w[2:], " "), n})
	case "line", "mark":
		kinds := map[string][]string{"line": {"arrow", "dotted", "planned"}, "mark": {"balloon", "flag", "diamond", "more"}}[kw]
		w := strings.Fields(rest)
		actor := ""
		if len(w) > 1 && strings.HasPrefix(w[1], "@") {
			actor, w = w[1][1:], slices.Delete(w, 1, 2)
		}
		if len(w) < 2 {
			p.fail(n, fmt.Errorf("%w: %s takes a kind and words", ErrFields, kw))
			return
		}
		if !slices.Contains(kinds, w[0]) {
			p.fail(n, fmt.Errorf("%w: %s %q, not one of %s", ErrUnknownMark, kw, w[0], strings.Join(kinds, ", ")))
			return
		}
		decl := legendDecl{w[0], actor, strings.Join(w[1:], " "), n}
		if kw == "line" {
			d.lines = append(d.lines, decl)
		} else {
			d.marks = append(d.marks, decl)
		}
	case "view":
		f, ok := p.fields(kw, rest, n, 1, 3)
		if !ok {
			return
		}
		num, title, _ := strings.Cut(f[0].s, " ")
		d.blocks = append(d.blocks, &viewHead{text{num, n}, text{strings.TrimSpace(title), n}, f[1], f[2]})
	default:
		p.unknown(kw, n)
	}
}

func (p *parser) titleblock(b *titleblock, kw string, t text) {
	if kw == "cell" {
		f, ok := p.fields(kw, t.s, t.line, 2, 2)
		if !ok {
			return
		}
		if !slices.Contains([]string{"id", "problem", "answer", "attn"}, f[0].s) {
			p.fail(t.line, fmt.Errorf("%w: cell %q, not one of id, problem, answer, attn", ErrUnknownBlock, f[0].s))
			return
		}
		b.cells = append(b.cells, &cell{kind: f[0].s, label: f[1]})
		return
	}
	if len(b.cells) == 0 {
		p.fail(t.line, fmt.Errorf("%w: %s before the first cell", ErrUnknownKeyword, kw))
		return
	}
	c := b.cells[len(b.cells)-1]
	switch kw {
	case "wordmark", "motto", "para", "lead", "big":
		c.els = append(c.els, &cellEl{kind: kw, text: t})
	case "point", "item":
		list, want := "plist", 3
		if kw == "item" {
			list, want = "ol", 2
		}
		f, ok := p.fields(kw, t.s, t.line, 2, want)
		if !ok {
			return
		}
		if len(c.els) == 0 || c.els[len(c.els)-1].kind != list {
			c.els = append(c.els, &cellEl{kind: list})
		}
		el := c.els[len(c.els)-1]
		el.rows = append(el.rows, [3]text(append(f, text{})[:3]))
		if n, ok := strings.CutPrefix(f[0].s, "◆"); kw == "item" && ok && n != "" && leading(n, isDigit) == n {
			p.doc.diamonds[n] = true
		}
	default:
		p.unknown(kw, t.line)
	}
}

func (p *parser) details(b *details, kw string, t text) {
	last := func() *panel {
		if len(b.panels) == 0 {
			return nil
		}
		return b.panels[len(b.panels)-1]
	}
	switch kw {
	case "detail":
		if f, ok := p.fields(kw, t.s, t.line, 2, 3); ok {
			b.panels = append(b.panels, &panel{tag: f[0], title: f[1], note: f[2]})
			p.doc.details[f[0].s] = true
		}
	case "notes":
		if f, ok := p.fields(kw, t.s, t.line, 2, 2); ok {
			b.panels = append(b.panels, &panel{notes: true, title: f[1]})
		}
	case "rung":
		pn := last()
		if pn == nil || pn.notes {
			p.fail(t.line, fmt.Errorf("%w: rung outside a detail", ErrUnknownKeyword))
			return
		}
		if f, ok := p.fields(kw, t.s, t.line, 4, 4); ok {
			actor, planned, _ := attrs(f[0])
			pn.rungs = append(pn.rungs, &rung{actor, planned, f[1], f[2], f[3]})
		}
	case "note", "foot":
		pn := last()
		if pn == nil || !pn.notes {
			p.fail(t.line, fmt.Errorf("%w: %s outside notes", ErrUnknownKeyword, kw))
			return
		}
		if kw == "foot" {
			pn.foot = t
			return
		}
		if f, ok := p.fields(kw, t.s, t.line, 2, 3); ok {
			pn.items = append(pn.items, [3]text(f))
			p.doc.notes[f[0].s] = true
		}
	default:
		p.unknown(kw, t.line)
	}
}

func (p *parser) assembly(b *assembly, kw string, t text) {
	switch kw {
	case "parts":
		if f, ok := p.fields(kw, t.s, t.line, 1, 2); ok {
			b.title, b.sub = f[0], f[1]
		}
	case "part":
		f, ok := p.fields(kw, t.s, t.line, 4, 4)
		if !ok {
			return
		}
		actor, planned, w := attrs(f[0])
		if len(w) != 1 {
			p.fail(t.line, fmt.Errorf("%w: part takes one number before its @actor", ErrFields))
			return
		}
		b.parts = append(b.parts, &part{text{w[0], t.line}, actor, planned, f[1], f[2], f[3]})
		p.doc.parts[w[0]] = actor.s
	case "box", "store", "person", "rail", "group":
		p.piece(b, kw, t)
	case "valve":
		p.valve(b, t)
	case "arrow":
		if a := p.arrow(t, true); a != nil {
			b.arrows = append(b.arrows, a)
		}
	default:
		p.unknown(kw, t.line)
	}
}

var cells = regexp.MustCompile(`^([A-Z])([1-9][0-9]*)(?:-([A-Z])([1-9][0-9]*))?$`)

func (p *parser) piece(b *assembly, kw string, t text) {
	n := t.line
	f := splitFields(t.s)
	actor, planned, words := attrs(text{f[0], n})
	it := &item{kind: kw, lane: actor, planned: planned, lines: trimmed(f[1:], n), line: n}
	if len(words) < 2 || actor.s == "" && kw != "group" || len(it.lines) == 0 {
		p.fail(n, fmt.Errorf("%w: %s takes a name, %s and cells such as B2 or C3-C5, then its lines", ErrFields, kw, map[bool]string{true: "an optional @actor", false: "an @actor"}[kw == "group"]))
		return
	}
	if (kw == "group" || kw == "rail") && len(it.lines) != 1 {
		p.fail(n, fmt.Errorf("%w: a %s takes one line, for its %s", ErrFields, kw, map[bool]string{true: "tab", false: "label"}[kw == "group"]))
		return
	}
	if kw == "group" && planned {
		p.fail(n, fmt.Errorf("%w: a group is not planned; its parts are", ErrUnknownMark))
		return
	}
	it.name = words[0]
	m := cells.FindStringSubmatch(words[1])
	if m == nil {
		p.fail(n, fmt.Errorf("%w: %q is no cell or span of cells, such as B2 or C3-C5", ErrFields, words[1]))
		return
	}
	it.col0, it.row0 = int(m[1][0]-'A'+1), atoi(m[2])
	it.col1, it.row1 = it.col0, it.row0
	if m[3] != "" {
		it.col1, it.row1 = int(m[3][0]-'A'+1), atoi(m[4])
	}
	if it.col1 < it.col0 || it.row1 < it.row0 {
		p.fail(n, fmt.Errorf("%w: %q runs backwards; write its top left cell first", ErrFields, words[1]))
		return
	}
	for _, w := range words[2:] {
		if kw != "box" || w != "stack" {
			p.fail(n, fmt.Errorf("%w: %s %q; a box may be a stack, and a part planned", ErrUnknownMark, kw, w))
			return
		}
		it.kind = w
	}
	if _, dup := b.named[it.name]; dup || b.valves[it.name] != nil {
		p.fail(n, fmt.Errorf("%w: %s %q", ErrDuplicate, kw, it.name))
		return
	}
	b.named[it.name] = it
	b.items = append(b.items, it)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (p *parser) arrow(t text, assembly bool) *arrow {
	n := t.line
	f := splitFields(t.s)
	w := strings.Fields(f[0])
	if len(w) < 2 || !assembly && (len(w) > 3 || len(f) > 1) {
		p.fail(n, fmt.Errorf("%w: arrow takes what it leaves, what it reaches and a style", ErrFields))
		return nil
	}
	a := &arrow{from: text{w[0], n}, to: text{w[1], n}, line: n, label: trimmed(f[1:], n)}
	for i := 2; i < len(w); i++ {
		switch {
		case slices.Contains([]string{"dotted", "planned"}, w[i]) && a.style == "":
			a.style = w[i]
		case assembly && strings.HasPrefix(w[i], "@") && a.actor.s == "":
			a.actor = text{w[i][1:], n}
		case assembly && w[i] == "via" && i+1 < len(w) && a.via.s == "":
			a.via = text{w[i+1], n}
			i++
		case assembly:
			p.fail(n, fmt.Errorf("%w: arrow %q, not dotted, planned, an @actor or via a valve", ErrUnknownMark, w[i]))
			return nil
		default:
			p.fail(n, fmt.Errorf("%w: arrow %q, not dotted or planned", ErrUnknownMark, w[i]))
			return nil
		}
	}
	return a
}

func (p *parser) valve(b *assembly, t text) {
	n := t.line
	f := splitFields(t.s)
	actor, planned, words := attrs(text{f[0], n})
	if len(words) != 1 || actor.s == "" || planned || len(f) > 2 {
		p.fail(n, fmt.Errorf("%w: valve takes a name and an @actor, then its marks", ErrFields))
		return
	}
	if _, dup := b.named[words[0]]; dup {
		p.fail(n, fmt.Errorf("%w: valve %q", ErrDuplicate, words[0]))
		return
	}
	if _, dup := b.valves[words[0]]; dup {
		p.fail(n, fmt.Errorf("%w: valve %q", ErrDuplicate, words[0]))
		return
	}
	v := &valve{name: words[0], lane: actor, line: n}
	if len(f) == 2 {
		v.marks = text{strings.TrimSpace(f[1]), n}
	}
	b.valves[v.name] = v
}

func (s *spec) last() *howto {
	if len(s.cols) == 0 || len(s.cols[len(s.cols)-1]) == 0 {
		return nil
	}
	col := s.cols[len(s.cols)-1]
	return col[len(col)-1]
}

func (p *parser) spec(b *spec, kw string, t text) {
	switch kw {
	case "column":
		b.cols = append(b.cols, nil)
	case "howto":
		if len(b.cols) == 0 {
			p.fail(t.line, fmt.Errorf("%w: howto before the first column", ErrUnknownKeyword))
			return
		}
		if f, ok := p.fields(kw, t.s, t.line, 2, 3); ok {
			i := len(b.cols) - 1
			h := &howto{no: f[0], to: f[1], typed: f[2]}
			b.cols[i] = append(b.cols[i], h)
			p.more = &h.more
		}
	case "caption":
		h := b.last()
		if h == nil {
			p.fail(t.line, fmt.Errorf("%w: caption before the first howto", ErrUnknownKeyword))
			return
		}
		h.content = append(h.content, t)
		p.more = &h.more
	default:
		p.unknown(kw, t.line)
	}
}

var place = regexp.MustCompile(`^([^.\s]+)(?:\.([1-9][0-9]*)(?:-([1-9][0-9]*))?)?$`)

func (p *parser) swimlane(b *swimlane, kw string, t text) {
	n := t.line
	switch kw {
	case "lane":
		f := splitFields(t.s)
		actor, _, words := attrs(text{f[0], n})
		if actor.s == "" || len(words) > 0 {
			p.fail(n, fmt.Errorf("%w: lane takes an @actor, then the lines under its name", ErrFields))
			return
		}
		if slices.ContainsFunc(b.lanes, func(l *lane) bool { return l.actor.s == actor.s }) {
			p.fail(n, fmt.Errorf("%w: lane @%s", ErrDuplicate, actor.s))
			return
		}
		b.lanes = append(b.lanes, &lane{actor, trimmed(f[1:], n)})
	case "phase":
		f, ok := p.fields(kw, t.s, n, 2, 2)
		if !ok {
			return
		}
		if slices.ContainsFunc(b.phases, func(ph *phase) bool { return ph.n.s == f[0].s }) {
			p.fail(n, fmt.Errorf("%w: phase %s", ErrDuplicate, f[0].s))
			return
		}
		b.phases = append(b.phases, &phase{f[0], f[1]})
	case "box", "text":
		f := splitFields(t.s)
		actor, planned, words := attrs(text{f[0], n})
		it := &item{kind: kw, lane: actor, lines: trimmed(f[1:], n), line: n}
		if kw == "box" && len(words) > 0 {
			it.name, words = words[0], words[1:]
		}
		if actor.s == "" || len(words) == 0 || len(it.lines) == 0 || kw == "box" && it.name == "" {
			p.fail(n, fmt.Errorf("%w: %s takes %s@lane and a place such as 2.1-3, then its lines", ErrFields, kw, map[string]string{"box": "a name, ", "text": ""}[kw]))
			return
		}
		m := place.FindStringSubmatch(words[0])
		if m == nil {
			p.fail(n, fmt.Errorf("%w: %q is no place: a phase, then .column or .first-last", ErrFields, words[0]))
			return
		}
		it.phase = text{m[1], n}
		it.col0, _ = strconv.Atoi(cmp.Or(m[2], "1"))
		it.col1, _ = strconv.Atoi(cmp.Or(m[3], m[2], "1"))
		if it.col1 < it.col0 {
			p.fail(n, fmt.Errorf("%w: %q spans no column", ErrFields, words[0]))
			return
		}
		if kw == "text" && planned {
			p.fail(n, fmt.Errorf("%w: a text is not planned, since it is drawn without a box; mark the box it speaks of", ErrUnknownMark))
			return
		}
		it.planned = planned
		for _, w := range words[1:] {
			if kw != "box" || it.kind != "box" || !slices.Contains([]string{"chip", "note", "stack"}, w) {
				p.fail(n, fmt.Errorf("%w: %s %q; a box may be one of chip, note or stack, and planned", ErrUnknownMark, kw, w))
				return
			}
			it.kind = w
		}
		if it.name != "" {
			if _, dup := b.boxes[it.name]; dup {
				p.fail(n, fmt.Errorf("%w: box %q", ErrDuplicate, it.name))
				return
			}
			b.boxes[it.name] = it
			p.more = &it.more
		}
		b.items = append(b.items, it)
	case "arrow":
		if a := p.arrow(t, false); a != nil {
			b.arrows = append(b.arrows, a)
		}
	case "detail":
		f, ok := p.fields(kw, t.s, n, 2, 2)
		if !ok {
			return
		}
		fr := &frame{letter: f[0]}
		for w := range strings.FieldsSeq(f[1].s) {
			fr.boxes = append(fr.boxes, text{w, n})
		}
		b.frames = append(b.frames, fr)
	default:
		p.unknown(kw, n)
	}
}

func trimmed(fields []string, n int) []text {
	ts := make([]text, len(fields))
	for i, f := range fields {
		ts[i] = text{strings.TrimSpace(f), n}
	}
	return ts
}
