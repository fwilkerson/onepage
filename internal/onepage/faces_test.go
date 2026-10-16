package onepage

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestAThemedSheetMeasuresItsLargerSizes(t *testing.T) {
	t.Parallel()
	for class, ratio := range map[string]float64{
		"ts":  13 / 11.5 * (1 + themeFaces.sans),
		"tb":  13.5 / 12 * (1 + themeFaces.sans),
		"":    13.5 / 12 * (1 + themeFaces.sans),
		"tc":  13.0 / 12 * (1 + themeFaces.tech),
		"tcl": 18.0 / 17 * (1 + themeFaces.tech),
		"tm":  12.5 / 11.5,
	} {
		got := textWidth("Runs a gate", class, themeFaces) / textWidth("Runs a gate", class, paperFaces)
		if math.Abs(got-ratio) > 1e-9 {
			t.Errorf("a themed %q run is %.4f of the paper's width, want %.4f", class, got, ratio)
		}
	}
}

func TestAFlagParsesItsActor(t *testing.T) {
	t.Parallel()
	d, errs := parse("s.txt", []byte("actor repo amber the repository\nmark flag @repo asked of a repository\nmark flag asked of anyone\n"))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if len(d.marks) != 2 || d.marks[0] != (legendDecl{"flag", "repo", "asked of a repository", 2}) || d.marks[1].actor != "" {
		t.Errorf("marks = %+v, want a flag @repo and a flag of no actor", d.marks)
	}
}

func TestAThemedDottedLineIsAsHeavyAsItsHeadsRoom(t *testing.T) {
	t.Parallel()
	_, themed, ok := strings.Cut(pageTmpl, "{{- with .Theme}}")
	if !ok {
		t.Fatal("page.tmpl has no themed block")
	}
	m := regexp.MustCompile(`svg \.d \{ stroke-width: ([\d.]+);`).FindStringSubmatch(themed)
	if m == nil {
		t.Fatal("the themed block sets no dotted line's stroke")
	}
	if w, _ := strconv.ParseFloat(m[1], 64); w != themeFaces.dotted || w <= paperFaces.dotted {
		t.Errorf("a themed dotted line is %vpx, the renderer keeps room for %v and the paper's is %v", w, themeFaces.dotted, paperFaces.dotted)
	}
	if paperFaces.dotted != strokes["d"] {
		t.Errorf("a paper dotted line keeps room for %v, its stroke is %v", paperFaces.dotted, strokes["d"])
	}
}
