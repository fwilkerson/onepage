package onepage_test

import (
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fwilkerson/onepage/internal/onepage"
)

func headLength(class string) float64 {
	f := strings.Fields(class)
	switch {
	case slices.Contains(f, "heavy"):
		return 9
	case slices.Contains(f, "d"):
		return 6.5 * 2
	case slices.Contains(f, "ph"):
		return 6.5 * 1.3
	}
	return 6.5 * 1.4
}

var (
	headedRe = regexp.MustCompile(`<path d="([^"]+)" class="([^"]+)"((?: marker-(?:start|end)="[^"]+")+)`)
	pathStep = regexp.MustCompile(`([MHVQA])([^MHVQA]*)`)
	pathNum  = regexp.MustCompile(`-?[\d.]+`)
)

func bentHeads(t *testing.T, page string) []string {
	t.Helper()
	var bent []string
	for svg := range strings.SplitSeq(page, "<svg") {
		if strings.HasPrefix(svg, ` class="swimlane"`) || strings.HasPrefix(svg, ` class="assembly"`) {
			bent = append(bent, bentHeadsIn(t, svg)...)
		}
	}
	return bent
}

func bentHeadsIn(t *testing.T, page string) []string {
	t.Helper()
	type piece struct {
		curve        bool
		x, y, dx, dy float64
	}
	parse := func(d string) []piece {
		var pieces []piece
		var x, y float64
		for _, s := range pathStep.FindAllStringSubmatch(d, -1) {
			var v []float64
			for _, n := range pathNum.FindAllString(s[2], -1) {
				f, err := strconv.ParseFloat(n, 64)
				if err != nil {
					t.Fatal(err)
				}
				v = append(v, f)
			}
			nx, ny := x, y
			switch s[1] {
			case "M":
				x, y = v[0], v[1]
				continue
			case "H":
				nx = v[0]
			case "V":
				ny = v[0]
			default:
				nx, ny = v[len(v)-2], v[len(v)-1]
			}
			pieces = append(pieces, piece{s[1] == "Q" || s[1] == "A", x, y, nx - x, ny - y})
			x, y = nx, ny
		}
		return pieces
	}
	var lines [][]piece
	for _, m := range regexp.MustCompile(`<path d="(M[^"]+)" class="(?:w|d|ph)[ "]`).FindAllStringSubmatch(page, -1) {
		lines = append(lines, parse(m[1]))
	}
	type room struct {
		x0, y0, x1, y1 float64
		d              string
	}
	var rooms []room
	var bent []string
	for _, m := range headedRe.FindAllStringSubmatch(page, -1) {
		pieces := parse(m[1])
		run := func(ps []piece) float64 {
			if ps[0].curve {
				return 0
			}
			d := math.Hypot(ps[0].dx, ps[0].dy)
			for _, p := range ps[1:] {
				if p.curve || math.Abs(p.dx*ps[0].dy-p.dy*ps[0].dx) > 1e-9 || p.dx*ps[0].dx+p.dy*ps[0].dy < 0 {
					break
				}
				d += math.Hypot(p.dx, p.dy)
			}
			return d
		}
		whole := 0.0
		for _, p := range pieces {
			whole += math.Hypot(p.dx, p.dy)
		}
		need := 1.9 * headLength(m[2])
		back := slices.Clone(pieces)
		slices.Reverse(back)
		end, start := run(back), run(pieces)
		twoHeads := strings.Contains(m[3], "marker-start")
		if end < whole && (end < need || twoHeads && start < need) {
			bent = append(bent, m[1]+" ("+m[2]+")")
			continue
		}
		last, first := pieces[len(pieces)-1], pieces[0]
		ex, ey := last.x+last.dx, last.y+last.dy
		n, k := math.Hypot(last.dx, last.dy), min(need, end)
		rooms = append(rooms, room{ex - last.dx/n*k, ey - last.dy/n*k, ex, ey, m[1]})
		if twoHeads {
			n, k := math.Hypot(first.dx, first.dy), min(need, start)
			rooms = append(rooms, room{first.x, first.y, first.x + first.dx/n*k, first.y + first.dy/n*k, m[1]})
		}
	}
	for _, r := range rooms {
		lo, hi := point2{min(r.x0, r.x1), min(r.y0, r.y1)}, point2{max(r.x0, r.x1), max(r.y0, r.y1)}
		for _, l := range lines {
			for _, p := range l {
				plo := point2{min(p.x, p.x+p.dx), min(p.y, p.y+p.dy)}
				phi := point2{max(p.x, p.x+p.dx), max(p.y, p.y+p.dy)}
				if lo.x == hi.x && plo.y == phi.y && plo.x < lo.x && lo.x < phi.x && lo.y < plo.y && plo.y < hi.y ||
					lo.y == hi.y && plo.x == phi.x && plo.y < lo.y && lo.y < phi.y && lo.x < plo.x && plo.x < hi.x {
					bent = append(bent, "a line crosses the room before the head of "+r.d)
				}
			}
		}
	}
	return bent
}

type point2 struct{ x, y float64 }

func render(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := onepage.Render(path, src)
	if err != nil {
		t.Fatalf("Render %s: %v", path, err)
	}
	return string(page)
}

func TestArrowsArriveStraight(t *testing.T) {
	t.Parallel()
	for name, page := range map[string]func(*testing.T) string{
		"kiln's sheet":            renderKiln,
		"memo's page":             func(t *testing.T) string { return render(t, "testdata/memo.txt") },
		"the page before 61f2e93": func(t *testing.T) string { return render(t, "testdata/before-61f2e93.txt") },
		"a bundle's branches": func(t *testing.T) string {
			return assemblyFrom(t, "box a @up A1-A3 | a\nbox b @up B1 | b\nbox c @up B2 | c\nbox d @up B3 | d\n"+
				"arrow a b\narrow a c\narrow a d\n")
		},
		"a swimlane's arrows spread low on a box above the one they reach": func(t *testing.T) string {
			return swimlaneFrom(t, "phase 1 | P\nphase 2 | Q\nbox a @up 1 | A | two\nbox c @up 2 | C\nbox b @up 2 | B\n"+
				"arrow a b\narrow a b\narrow a b\narrow a b\n")
		},
		"a way around the part another way reaches": func(t *testing.T) string {
			return assemblyFrom(t, "box top @up B1 | top\nbox bottom @up B3 | bottom\nbox left @up A2 | left\n"+
				"box right @up B2-C2 | right\nbox pad @up C1 | pad\narrow left right\narrow top bottom\n")
		},
		"a dotted way that turns": func(t *testing.T) string {
			return assemblyFrom(t, "box a @up A1 | a\nbox b @up B2 | b\narrow a b dotted\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if bent := bentHeads(t, page(t)); len(bent) > 0 {
				t.Errorf("these arrows turn or curve under or just before their heads:\n%s", strings.Join(bent, "\n"))
			}
		})
	}
}
