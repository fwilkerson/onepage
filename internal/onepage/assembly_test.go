package onepage_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/fwilkerson/onepage/internal/onepage"
)

func assemblyOf(t *testing.T, page string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<svg class="assembly"`)
	if !ok {
		t.Fatal("the page draws no assembly")
	}
	svg, _, _ := strings.Cut(rest, "</svg>")
	return svg
}

func TestRenderDrawsAnAssembly(t *testing.T) {
	t.Parallel()
	svg := assemblyOf(t, renderKiln(t))

	const n = `[\d.]+`
	for _, want := range []string{
		`^ width="960" height="` + n + `" viewBox="0 0 960 ` + n + `">`,
		`<circle cx="` + n + `" cy="` + n + `" r="15" class="bx maker"/>`,
		`<path d="M[^"]+Z" class="bx maker"/>`,
		`class="tc f-maker">POTTER</text>`,
		`class="ts">at the wheel</text>`,
		`<rect x="` + n + `" y="` + n + `" width="` + n + `" height="` + n + `" class="bx oven"/>`,
		`class="tm">kiln fire</text>`,
		`<g class="stack"><rect x="` + n + `" y="` + n + `" width="` + n + `" height="` + n + `" class="bx oven"/><rect `,
		`class="bx shelf ph"/>`,
		`<rect x="` + n + `" y="` + n + `" width="` + n + `" height="` + n + `" class="grp shelf"/>`,
		`<rect x="` + n + `" y="` + n + `" width="` + n + `" height="16" class="tab"/>`,
		`class="tc f-shelf">STORE ROOM</text>`,
		`<path d="M[^"]+A[^"]+Z" class="bx log"/><ellipse cx="` + n + `" cy="` + n + `" rx="` + n + `" ry="` + n + `" class="bx log"/>`,
		`class="tc f-log">FIRING LOG</text>`,
		`<path d="M[^"]+" class="w maker" marker-end="url\(#a-maker\)"/>`,
		`<path d="M[^"]+" class="ph oven" marker-end="url\(#a-oven\)"/>`,
		`<path d="M[^"]+" class="d shelf" marker-start="url\(#a-shelf\)" marker-end="url\(#a-shelf\)"/>`,
		`<path d="M[^"]+" class="w oven" marker-end="url\(#a-oven\)"/>`,
		`<g class="bal log" data-item="1" transform="translate\(` + n + `,` + n + `\)"><circle r="10"/><text y="4">1</text></g>`,
		`<g class="bal shelf" data-item="2" transform=`,
		`<g class="flag" data-note="1" transform=`,
		`class="pip oven"/><text x="` + n + `" y="` + n + `" class="pipt oven">C6</text>`,
		`class="leaf oven"/><text x="` + n + `" y="` + n + `" class="ts">cone chart</text>`,
		`class="bx cmp"/><text x="` + n + `" y="` + n + `" class="tc">FIRINGS</text>`,
		`<path d="M` + n + `,` + n + ` V` + n + `" class="w heavy shelf"/>`,
		`class="valve oven"/>`,
		`class="ts">read back</text>`,
	} {
		if !regexp.MustCompile(want).MatchString(svg) {
			t.Errorf("the assembly lacks %s", want)
		}
	}
}

func assemblyFrom(t *testing.T, body string) string {
	t.Helper()
	page, err := onepage.Render("s.txt", []byte("actor up amber up\n::: assembly\n"+body+":::\n"))
	if err != nil {
		t.Fatal(err)
	}
	return assemblyOf(t, string(page))
}

func TestAssemblyHopsWhereConnectionsCross(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box top @up B1 | top\nbox bottom @up B3 | bottom\n"+
		"box left @up A2 | left\nbox right @up C2 | right\narrow top bottom\narrow left right\n")
	paths := regexp.MustCompile(`<path d="([^"]+)" class="w up"`).FindAllStringSubmatch(svg, -1)
	if len(paths) != 2 {
		t.Fatalf("paths = %q, want two", paths)
	}
	hop := regexp.MustCompile(`H[\d.]+ A4,4 0 0,1 [\d.]+,[\d.]+`)
	if hop.MatchString(paths[0][1]) || !hop.MatchString(paths[1][1]) {
		t.Errorf("paths = %q, want a hop on the second only", paths)
	}
}

func TestAssemblyRoutesAroundAPart(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nbox b @up B1 | b\nbox c @up C1 | c\narrow a c\n")
	m := regexp.MustCompile(`<path d="([^"]+)" class="w up"`).FindStringSubmatch(svg)
	if m == nil || strings.Count(m[1], "V") < 2 {
		t.Errorf("path = %q, want it to leave the row to pass b", m)
	}
}

func TestAssemblyRunsStraightToASpreadPort(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a | b | c | d | e\nbox b @up B1 | a | b | c | d | e\nbox c @up B2 | c\narrow a b\narrow a c\n")
	m := regexp.MustCompile(`<path d="([^"]+)" class="w up"`).FindStringSubmatch(svg)
	if m == nil || !regexp.MustCompile(`^M\d+,\d+ H\d+$`).MatchString(m[1]) {
		t.Errorf("path = %q, want it straight across", m)
	}
}

func TestAssemblyNestsWaysThatNeedNotCross(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A3 | a\nbox b @up B1 | b\nbox c @up B2 | c\narrow a b\narrow a c\n")
	if strings.Contains(svg, " A4,4 ") {
		t.Errorf("the ways cross:\n%s", svg)
	}
}

func TestAssemblyPortsClearOfATab(t *testing.T) {
	t.Parallel()
	assemblyFrom(t, "box top @up B1 | top\ngroup g B2-C2 | A TAB LONG ENOUGH TO COVER HALF\nbox m @up B2 | m\nbox n @up C2 | n\narrow top m\n")
}

func TestAssemblyMakesRoomForItsPorts(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("box a @up A1 | a\n")
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(&b, "box b%d @up B%d | b\narrow a b%d\n", i, i, i)
	}
	assemblyFrom(t, b.String())
}

func TestAssemblyMeetsAStoreUnderItsLid(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nbox b @up B1 | b\nbox c @up C1 | c\nstore s @up A2-C2 | s\narrow a s\n")
	lid := regexp.MustCompile(`<ellipse cx="(\d+)" cy="\d+" rx="(\d+)"`).FindStringSubmatch(svg)
	ends := regexp.MustCompile(`[MH](\d+)(?:,\d+)? V\d+" class="w up"`).FindAllStringSubmatch(svg, -1)
	if lid == nil || len(ends) != 1 {
		t.Fatalf("lid = %q, ends = %q", lid, ends)
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	cx, rx := n(lid[1]), n(lid[2])
	for _, e := range ends {
		if x := n(e[1]); x < cx-rx/2 || x > cx+rx/2 {
			t.Errorf("a way reaches the store at x %d, outside the middle of its lid, %d to %d", x, cx-rx/2, cx+rx/2)
		}
	}
}

func TestAssemblyStandsAPersonInTheMiddleOfItsCells(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "person p @up A1-A3 | P\nbox a @up B1 | a\nbox b @up B3 | b\n")
	head := regexp.MustCompile(`<circle cx="\d+" cy="(\d+)" r="15"`).FindStringSubmatch(svg)
	boxes := regexp.MustCompile(`<rect x="\d+" y="(\d+)" width="\d+" height="(\d+)" class="bx up"/>`).FindAllStringSubmatch(svg, -1)
	if head == nil || len(boxes) != 2 {
		t.Fatalf("head = %q, boxes = %q", head, boxes)
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	if cy := n(head[1]); cy <= n(boxes[0][1])+n(boxes[0][2]) || cy >= n(boxes[1][1]) {
		t.Errorf("the head is at %d, want it between the box ending at %d and the one starting at %d", cy, n(boxes[0][1])+n(boxes[0][2]), n(boxes[1][1]))
	}
}

func ints(t *testing.T, re, s string) []int {
	t.Helper()
	var vs []int
	for _, m := range regexp.MustCompile(re).FindAllStringSubmatch(s, -1) {
		for _, g := range m[1:] {
			v, err := strconv.Atoi(g)
			if err != nil {
				t.Fatal(err)
			}
			vs = append(vs, v)
		}
	}
	return vs
}

const parts = "part 1 @up | one | a | b\npart 2 @up | two | a | b\npart 3 @up | three | a | b\n"

func TestAssemblyDrawsAStoresCompartments(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "store s @up A1 | S | one line | [ONE {1}] [TWO] | [THREE] [FOUR]\n"+parts)
	store := ints(t, `<path d="M(\d+),(\d+) V(\d+) A\d+,\d+ 0 0,0 (\d+),`, svg)
	cmps := ints(t, `<rect x="(\d+)" y="(\d+)" width="(\d+)" height="(\d+)" class="bx cmp"/>`, svg)
	if len(store) != 4 || len(cmps) != 16 {
		t.Fatalf("store = %v, compartments = %v, want one store and four", store, cmps)
	}
	for i := 0; i < 16; i += 4 {
		if x, y, w, h := cmps[i], cmps[i+1], cmps[i+2], cmps[i+3]; x <= store[0] || x+w >= store[3] || y <= store[1] || y+h >= store[2] {
			t.Errorf("compartment at %d,%d %dx%d lies outside the store %v", x, y, w, h, store)
		}
	}
	bal := ints(t, `<g class="bal up" data-item="1" transform="translate\((\d+),(\d+)\)"><circle`, svg)
	if len(bal) != 2 || bal[0] <= cmps[0]+cmps[2]/2 || bal[0] >= cmps[0]+cmps[2] || bal[1] <= cmps[1] || bal[1] >= cmps[1]+cmps[3] {
		t.Errorf("balloon 1 at %v, want it in the right half of the first compartment %v", bal, cmps[:4])
	}
	if strings.Contains(svg, ">[") {
		t.Errorf("a chip's brackets are drawn:\n%s", svg)
	}
}

func TestAssemblyCallsOutBalloons(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | A {1} | gates [F][P] {2} | more words\n"+parts)
	box := ints(t, `<rect x="(\d+)" y="(\d+)" width="(\d+)" height="(\d+)" class="bx up"/>`, svg)
	chips := ints(t, `<rect x="(\d+)" y="\d+" width="(\d+)" height="\d+" class="pip up"/>`, svg)
	if len(box) != 4 || len(chips) != 4 {
		t.Fatalf("box = %v, chips = %v, want one box and two chips", box, chips)
	}
	right := box[0] + box[2]
	one := ints(t, `<g class="bal up" data-item="1" transform="translate\((\d+),(\d+)\)"><circle`, svg)
	if len(one) != 2 || abs(one[0]-right) > 4 || abs(one[1]-box[1]) > 4 {
		t.Errorf("balloon 1 at %v, want it on the corner %d,%d", one, right, box[1])
	}
	two := ints(t, `<g class="bal up" data-item="2" transform="translate\((\d+),(\d+)\)"><line x1="0" y1="0" x2="(-\d+)" y2="0"/><circle`, svg)
	if len(two) != 3 || two[0] <= right+4 || two[0]+two[2] != chips[2]+chips[3]+2 {
		t.Errorf("balloon 2 = %v, want it past the right edge %d on a leader to the last chip's end %d", two, right, chips[2]+chips[3])
	}
}

func abs(v int) int { return max(v, -v) }

func TestAssemblyFansOutFromOneSide(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1-A3 | a\nbox b @up B1 | b\nbox c @up B2 | c\nbox d @up B3 | d\narrow a b\narrow a c\narrow a d\n")
	box := ints(t, `<rect x="(\d+)" y="\d+" width="(\d+)" height="\d+" class="bx up"/>`, svg)
	starts := ints(t, `<path d="M(\d+),(\d+) [^"]*" class="w up"`, svg)
	dots := ints(t, `<circle cx="(\d+)" cy="(\d+)" r="3.5" class="dot up"/>`, svg)
	if len(starts) != 6 || len(dots) != 4 {
		t.Fatalf("starts = %v, dots = %v, want three ways and two junctions", starts, dots)
	}
	fromEdge := 0
	atDot := func(x, y int) bool { return x == dots[0] && y == dots[1] || x == dots[2] && y == dots[3] }
	for i := 0; i < 6; i += 2 {
		switch {
		case starts[i] == box[0]+box[1]:
			fromEdge++
		case !atDot(starts[i], starts[i+1]):
			t.Errorf("a way starts at %d,%d, neither at the part nor at a junction %v", starts[i], starts[i+1], dots)
		}
	}
	if fromEdge != 1 {
		t.Errorf("%d ways leave the part, want one trunk", fromEdge)
	}
}

func TestAssemblyEndsOnARail(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nbox b @up A2 | b\nrail r @up B1-B2 | R\nbox t @up C2 | t\narrow a r\narrow b r\narrow r t\n")
	rail := ints(t, `<path d="M(\d+),(\d+) V\d+ Q[^"]+" class="w heavy up" marker-end="url\(#a-heavy-up\)"/>`, svg)
	if len(rail) != 2 {
		t.Fatalf("no rail turning into t:\n%s", svg)
	}
	ways := ints(t, `<path d="M\d+,(\d+) H(\d+)" class="w up"/>`, svg)
	dots := ints(t, `<circle cx="(\d+)" cy="(\d+)" r="4.5" class="dot up"/>`, svg)
	if len(ways) != 4 || len(dots) != 4 {
		t.Fatalf("ways = %v, dots = %v, want two straight ways ending in dots", ways, dots)
	}
	for i := 0; i < 4; i += 2 {
		if dots[i] != rail[0] || ways[i+1] != rail[0] || dots[i+1] != ways[i] {
			t.Errorf("a way ends at %d,%d with a dot at %d,%d, want both on the rail at x %d", ways[i+1], ways[i], dots[i], dots[i+1], rail[0])
		}
	}
	if !strings.Contains(svg, `class="tc f-up">R</text>`) {
		t.Errorf("the rail's label is not drawn:\n%s", svg)
	}
}

func TestAssemblyDrawsAValveOnEachWayVia(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nbox b @up A2 | b\nbox c @up C1-C2 | c\nvalve v @up | {1}\narrow a c via v\narrow b c via v\n"+parts)
	if n := strings.Count(svg, `class="valve up"`); n != 2 {
		t.Errorf("%d valves drawn, want one on each way", n)
	}
	if n := strings.Count(svg, `data-item="1"`); n != 1 {
		t.Errorf("balloon 1 drawn %d times, want once", n)
	}
}

func TestAssemblyLabelsAConnection(t *testing.T) {
	t.Parallel()
	svg := page(t, "actor up amber up\nactor down cyan down\n::: assembly\nbox a @up A1 | a\nbox b @up C1 | b\narrow a b @down | first line | second\n:::\n")
	way := ints(t, `<path d="M\d+,(\d+) H\d+" class="w down" marker-end="url\(#a-down\)"/>`, svg)
	labels := ints(t, `<text x="\d+" y="(\d+)" class="ts">(?:first line|second)</text>`, svg)
	if len(way) != 1 || len(labels) != 2 {
		t.Fatalf("way = %v, labels = %v, want a straight way in down's colour and two label lines", way, labels)
	}
	if labels[0] < way[0] && way[0] < labels[1]+4 || labels[1]-labels[0] < 12 {
		t.Errorf("the way at y %d runs through its label at %v", way[0], labels)
	}
}

func TestAssemblyHeadsADottedWayAtBothEnds(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nbox b @up B1 | b\narrow a b dotted\n")
	if !strings.Contains(svg, `class="d up" marker-start="url(#a-up)" marker-end="url(#a-up)"/>`) {
		t.Errorf("the dotted way has no head at each end:\n%s", svg)
	}
}

func TestAssemblyReachesARailFromBelow(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nrail r @up B1-B2 | R\nbox c @up B3 | c\narrow a r\narrow c r\n")
	rail := ints(t, `<path d="M(\d+),\d+ V\d+" class="w heavy up"/>`, svg)
	dots := ints(t, `<circle cx="(\d+)" cy="\d+" r="4.5" class="dot up"/>`, svg)
	if len(rail) != 1 || len(dots) != 2 || dots[0] != rail[0] || dots[1] != rail[0] {
		t.Errorf("rail at %v, dots at %v, want both dots on the rail", rail, dots)
	}
}

func TestAssemblyHopsATrunkOnce(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A2-A4 | a\nbox b @up C2 | b\nbox c @up C3 | c\nbox d @up C4 | d\n"+
		"box l @up B1 | l\nbox m @up B5 | m\narrow a b\narrow a c\narrow a d\narrow l m\n")
	if n := strings.Count(svg, " A4,4 "); n > 1 {
		t.Errorf("%d hops, want at most one over the trunk:\n%s", n, svg)
	}
}

func TestAssemblySpreadsBalloonsOnOneLine(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | A {1} {2} | words {3} {1}\n"+parts)
	at := regexp.MustCompile(`<g class="bal up" data-item="\d" transform="translate\((\d+,\d+)\)"`).FindAllStringSubmatch(svg, -1)
	seen := map[string]bool{}
	for _, m := range at {
		if seen[m[1]] {
			t.Errorf("two balloons at %s", m[1])
		}
		seen[m[1]] = true
	}
	if len(at) != 4 {
		t.Errorf("%d balloons, want 4", len(at))
	}
}

func TestAssemblyRunsABranchStraightToItsPort(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1-A3 | a | b | c | d | e | f | g | h | i | j | k\nbox b @up B1 | b\nbox c @up B2 | c | d | e\nbox d @up B3 | d\narrow a b\narrow a c\narrow a d\n")
	if !regexp.MustCompile(`<path d="M\d+,(\d+) H\d+" class="w up"`).MatchString(svg) {
		t.Errorf("no way runs straight across:\n%s", svg)
	}
}

func TestAssemblyFacesPortsAcrossAGap(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "box a @up A1 | a\nbox b @up B1-B2 | b | b | b | b\nbox c @up A2 | c\narrow a b dotted\narrow c b dotted\n")
	paths := regexp.MustCompile(`<path d="([^"]+)" class="d up"`).FindAllStringSubmatch(svg, -1)
	if len(paths) != 2 {
		t.Fatalf("paths = %q, want two", paths)
	}
	for _, p := range paths {
		if !regexp.MustCompile(`^M\d+,\d+ H\d+$`).MatchString(p[1]) {
			t.Errorf("path = %q, want it straight across", p[1])
		}
	}
}

func TestAssemblyPlacesACalloutClearOfItsGroup(t *testing.T) {
	t.Parallel()
	svg := assemblyFrom(t, "group g A1 | G\nbox a @up A1 | A | words {1}\n"+parts)
	grp := ints(t, `<rect x="(\d+)" y="\d+" width="(\d+)" height="\d+" class="grp"/>`, svg)
	bal := ints(t, `<g class="bal up" data-item="1" transform="translate\((\d+),`, svg)
	if len(grp) != 2 || len(bal) != 1 || bal[0]-10 <= grp[0]+grp[1] {
		t.Errorf("group %v, balloon at x %v, want it past the group's right edge", grp, bal)
	}
}

func page(t *testing.T, src string) string {
	t.Helper()
	p, err := onepage.Render("s.txt", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return assemblyOf(t, string(p))
}
