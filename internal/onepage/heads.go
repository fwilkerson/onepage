package onepage

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

const (
	headStrokes = 6.5
	heavyHead   = 9
)

var strokes = map[string]float64{"w": 1.4, "ph": 1.3, "d": 2}

func (fc faces) headLength(class string) float64 {
	f := strings.Fields(class)
	switch {
	case slices.Contains(f, "heavy"):
		return heavyHead
	case f[0] == "d":
		return headStrokes * fc.dotted
	}
	return headStrokes * strokes[f[0]]
}

func (fc faces) room(class string) int {
	return int(math.Ceil(1.9 * fc.headLength(class)))
}

func (r *renderer) arrive(line int, d, class string, twoHeads bool, a, b string) {
	start, end, bent := straight(d)
	need := r.faces.room(class)
	switch {
	case !bent:
	case end < float64(need):
		r.fail(line, fmt.Errorf("%w: the arrow from %q to %q runs %.0fpx straight into its head at %q, which needs %d; move a part or leave more room between them", ErrHeadOnTurn, a, b, end, b, need))
	case twoHeads && start < float64(need):
		r.fail(line, fmt.Errorf("%w: the arrow from %q to %q runs %.0fpx straight out of its head at %q, which needs %d; move a part or leave more room between them", ErrHeadOnTurn, a, b, start, a, need))
	}
}

func straight(d string) (start, end float64, bent bool) {
	type piece struct {
		curve  bool
		dx, dy float64
	}
	var pieces []piece
	var x, y float64
	cmd, args := byte(0), []float64(nil)
	flush := func() {
		if cmd == 0 {
			return
		}
		nx, ny := x, y
		switch cmd {
		case 'M':
			x, y = args[0], args[1]
			return
		case 'H':
			nx = args[0]
		case 'V':
			ny = args[0]
		default:
			nx, ny = args[len(args)-2], args[len(args)-1]
		}
		pieces = append(pieces, piece{cmd == 'Q' || cmd == 'A', nx - x, ny - y})
		x, y = nx, ny
	}
	for f := range strings.FieldsSeq(d) {
		if c := f[0]; c >= 'A' && c <= 'Z' {
			flush()
			cmd, args, f = c, nil, f[1:]
		}
		for n := range strings.SplitSeq(f, ",") {
			if v, err := strconv.ParseFloat(n, 64); err == nil {
				args = append(args, v)
			}
		}
	}
	flush()

	run := func(ps []piece) float64 {
		if len(ps) == 0 || ps[0].curve {
			return 0
		}
		first, n := ps[0], 0.0
		for _, p := range ps {
			if p.curve || p.dx*first.dy != p.dy*first.dx || p.dx*first.dx+p.dy*first.dy < 0 {
				break
			}
			n += math.Hypot(p.dx, p.dy)
		}
		return n
	}
	whole := 0.0
	for _, p := range pieces {
		whole += math.Hypot(p.dx, p.dy)
	}
	back := slices.Clone(pieces)
	slices.Reverse(back)
	start, end = run(pieces), run(back)
	return start, end, end < whole
}
