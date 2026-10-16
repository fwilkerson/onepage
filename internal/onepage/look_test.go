package onepage_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/fwilkerson/onepage/internal/onepage"
)

func themedKiln(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/kiln.txt")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.NewReplacer(
		"title Kiln, on one sheet\n", "title Kiln, on one sheet\ntheme theme/sheet.css\n",
		"amber ", "you   ", "cyan  ", "agent ", "green ", "ref   ", "violet", "ink-2 ",
		"mark flag asked", "mark flag @shelf asked",
	).Replace(string(b))
	page, err := onepage.Render("testdata/kiln.txt", []byte(src))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(page)
}

func TestAThemedSheetIsSetLarger(t *testing.T) {
	t.Parallel()
	const src = "actor you %s the developer\n::: swimlane\nlane @you\nphase 1 | P\nbox a @you 1 | asks | and waits\n:::\n"
	box := regexp.MustCompile(`<rect x="\d+" y="38" width="\d+" height="(\d+)" class="bx you"/>\n  <text x="\d+" y="(\d+)" class="ts">asks</text>`)
	look := func(src string) []string {
		t.Helper()
		page, err := onepage.Render("testdata/s.txt", []byte(src))
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		m := box.FindStringSubmatch(string(page))
		if m == nil {
			t.Fatalf("page draws no box asks:\n%s", swimlaneOf(t, string(page)))
		}
		return m[1:]
	}
	if got := look(strings.Replace(src, "%s", "amber", 1)); got[0] != "39" || got[1] != "54" {
		t.Errorf("a paper box is %spx high with its first line at %s, want 39 and 54", got[0], got[1])
	}
	if got := look("theme theme/sheet.css\n" + strings.Replace(src, "%s", "you", 1)); got[0] != "43" || got[1] != "56" {
		t.Errorf("a themed box is %spx high with its first line at %s, want 43 and 56", got[0], got[1])
	}
	page := themedKiln(t)
	for _, want := range []string{
		`html, body { font: 400 14.5px/1.45 var(--sans); }`,
		`svg .ts { font-size: 13px; }`,
		`.tblock { grid-template-columns: 330px 400px 440px 1fr; }`,
		`.spec { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 24px; }`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("a themed page lacks\n%s", want)
		}
	}
	if strings.Contains(page, "PROTOTYPE") {
		t.Errorf("a themed page holds the prototype's note")
	}
}

func TestAThemedSheetKeepsCirclesForParts(t *testing.T) {
	t.Parallel()
	page := themedKiln(t)
	svg := swimlaneOf(t, page)
	if strings.Contains(svg, `cy="15" r="10" class="bx"`) {
		t.Errorf("a themed swimlane rings its phase numbers")
	}
	for _, want := range []string{`<text x="\d+" y="19" class="tc">1</text><text x="\d+" y="19" class="tc">LOAD</text>`, `rx="8" class="detail"/>`} {
		if !regexp.MustCompile(want).MatchString(svg) {
			t.Errorf("the swimlane lacks %s", want)
		}
	}
	for _, want := range []string{
		`.sh .n { display: inline; width: auto; height: auto; border: 0; border-radius: 0; font: inherit; letter-spacing: 0; }`,
		`svg .detail { stroke: var(--ink-2); stroke-width: 1; stroke-dasharray: none; }`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("a themed page lacks\n%s", want)
		}
	}
	if !strings.Contains(renderKiln(t), `cy="15" r="10" class="bx"/><text x="`) {
		t.Errorf("a paper swimlane no longer rings its phase numbers")
	}
}

func TestAThemedSheetDrawsAGreyActorsBoxesInInk(t *testing.T) {
	t.Parallel()
	page := themedKiln(t)
	for _, want := range []string{
		"  svg .tc.f-shelf, svg .tcl.f-shelf { fill: var(--ink); }\n  svg .bx.shelf { stroke: var(--line); }\n",
		"  .shelf { color: var(--shelf); }",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("a themed page lacks\n%s", want)
		}
	}
	if strings.Contains(page, "f-maker { fill: var(--ink)") || strings.Contains(page, "svg .bx.maker { stroke: var(--line)") {
		t.Errorf("a themed page draws an actor not in ink-2 as grey")
	}
}

func TestAFlagTakesItsActorsColour(t *testing.T) {
	t.Parallel()
	page := themedKiln(t)
	for _, want := range []string{
		"  .flag::before { background: var(--shelf); }\n  .flag::after { color: var(--shelf); }\n  svg .flag path { stroke: var(--shelf); }\n  svg .flag text { fill: var(--shelf); }\n",
		`<g class="flag" data-note="1" transform="translate(`,
		`"><path d="M0,-10 L11,10 L-11,10 Z"/><text y="8">1</text></g>`,
		`.flag { width: 22px; height: 20px; vertical-align: -4px; }`,
		`svg .flag text { font-size: 11px; }`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("a themed page lacks\n%s", want)
		}
	}
	paper := renderKiln(t)
	if strings.Contains(paper, "\n  .flag::before { background: var(--") || strings.Contains(paper, `L11,10`) {
		t.Errorf("a flag with no actor is drawn differently on paper")
	}
	src, err := os.ReadFile("testdata/kiln.txt")
	if err != nil {
		t.Fatal(err)
	}
	coloured, err := onepage.Render("testdata/kiln.txt", []byte(strings.Replace(string(src), "mark flag asked", "mark flag @shelf asked", 1)))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"  .flag::before { background: var(--shelf); }\n", "  svg .flag path { fill: var(--shelf); }\n"} {
		if !strings.Contains(string(coloured), want) {
			t.Errorf("a paper page with a flag @shelf lacks\n%s", want)
		}
	}
}
