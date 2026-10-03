package onepage_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fwilkerson/onepage/internal/onepage"
)

func swimlaneOf(t *testing.T, page string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<svg class="swimlane"`)
	if !ok {
		t.Fatal("the page draws no swimlane")
	}
	svg, _, _ := strings.Cut(rest, "</svg>")
	return svg
}

func TestRenderDrawsASwimlane(t *testing.T) {
	t.Parallel()
	svg := swimlaneOf(t, renderKiln(t))

	const n = `[\d.]+`
	for _, want := range []string{
		`^ width="1500" height="` + n + `" viewBox="0 0 1500 ` + n + `">`,
		`<rect x="0" y="30" width="1500" height="` + n + `" class="lane maker"/>`,
		`<rect x="0" y="` + n + `" width="5" height="` + n + `" class="f-oven"/>`,
		`<text x="16" y="` + n + `" class="tcl f-maker">MAKER</text><text x="16" y="` + n + `" class="ts">the potter</text>`,
		`class="tcl f-oven">OVEN</text><text x="16" y="` + n + `" class="ts">the kiln</text><text x="16" y="` + n + `" class="ts">itself</text>`,
		`<circle cx="` + n + `" cy="15" r="10" class="bx"/><text x="` + n + `" y="19" class="tc mid">1</text><text x="` + n + `" y="19" class="tc">LOAD</text>`,
		`<text x="` + n + `" y="19" class="tc">FIRE</text>`,
		`<path d="M` + n + `,4 V` + n + `" class="sep"/>`,
		`<rect x="` + n + `" y="38" width="` + n + `" height="` + n + `" class="bx maker"/>`,
		`class="tb">load the kiln</text>`,
		`class="tm">kiln load</text>`,
		`class="bx oven chip"/>`,
		`<g class="stack"><rect x="` + n + `" y="` + n + `" width="` + n + `" height="` + n + `" class="bx oven"/><rect `,
		`class="ts">×N, one by one</text>`,
		`class="tc f-log">FIRING LOG</text>`,
		`class="bx log ph"/>`,
		`<text x="` + n + `" y="` + n + `"><tspan class="ts">an </tspan><tspan class="ts f-log">alarm</tspan></text>`,
		`class="bx note"/>`,
		`class="ti">or wait for</text>`,
		`<g class="dia" transform="translate\(` + n + `,` + n + `\)"><path d="M0,-8 L8,0 L0,8 L-8,0 Z"/><text y="3.5">1</text></g>`,
		`<g class="flag" data-note="1" transform="translate\(` + n + `,` + n + `\)"><path d="M0,-8 L9,8 L-9,8 Z"/><text y="6">1</text></g>`,
		`<g class="bal log" data-item="1" transform="translate\(` + n + `,` + n + `\)"><circle r="10"/><text y="4">1</text></g>`,
		`<path d="M` + n + `,` + n + ` V` + n + `" class="w maker" marker-end="url\(#a-maker\)"/>`,
		`<path d="[^"]+" class="d oven"/>`,
		`<path d="[^"]+" class="ph log" marker-end="url\(#a-log\)"/>`,
		`<rect x="` + n + `" y="` + n + `" width="` + n + `" height="` + n + `" rx="8" class="detail"/>`,
		`<g class="dtag" transform="translate\(` + n + `,` + n + `\)"><circle r="11"/><text y="4.5">A</text></g>`,
	} {
		if !regexp.MustCompile(want).MatchString(svg) {
			t.Errorf("the swimlane lacks %s", want)
		}
	}
}

func swimlaneFrom(t *testing.T, body string) string {
	t.Helper()
	src := "actor up amber up\nactor down cyan down\n::: swimlane\nlane @up\nlane @down\n" + body + ":::\n"
	page, err := onepage.Render("s.txt", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return swimlaneOf(t, string(page))
}

func TestSwimlaneTurnsClearOfBoxes(t *testing.T) {
	t.Parallel()
	svg := swimlaneFrom(t, "phase 1 | P\n"+
		"box tall @up 1.1 | a | b | c | d | e\nbox short @up 1.2 | a | b\nbox far @down 1.3 | far\narrow tall far\n")
	if !regexp.MustCompile(`<path d="M\d+,\d+ V\d+ H\d+" class="w up"`).MatchString(svg) {
		t.Errorf("the arrow does not go down first:\n%s", svg)
	}
}

func TestSwimlaneSpreadsArrowsBetweenAPair(t *testing.T) {
	t.Parallel()
	svg := swimlaneFrom(t, "phase 1 | P\nbox a @up 1.1 | a\nbox b @down 1.2 | b\narrow a b\narrow a b dotted\n")
	paths := regexp.MustCompile(`<path d="([^"]+)" class="(?:w|d) up"`).FindAllStringSubmatch(svg, -1)
	if len(paths) != 2 || paths[0][1] == paths[1][1] {
		t.Errorf("paths = %q, want two apart", paths)
	}
}

func TestSwimlaneStandsATextCloseUnderABox(t *testing.T) {
	t.Parallel()
	svg := swimlaneFrom(t, "phase 1 | P\nbox a @up 1 | a\ntext @up 1 | aside\nbox b @up 1 | b\n")
	for _, want := range []string{
		`<rect x="\d+" y="38" width="\d+" height="24" class="bx up"/>`,
		`<text x="\d+" y="77" class="ti">aside</text>`,
		`<rect x="\d+" y="101" width="\d+" height="24" class="bx up"/>`,
	} {
		if !regexp.MustCompile(want).MatchString(svg) {
			t.Errorf("the swimlane lacks %s:\n%s", want, svg)
		}
	}
}

func TestSwimlaneKeepsALinesStyleInBoldAndCode(t *testing.T) {
	t.Parallel()
	svg := swimlaneFrom(t, "phase 1 | P\nbox a @up 1 | **FIRING** LOG | **bold** and `code`\ntext @down 1 | an **aside** in `code`\n")
	for _, want := range []string{
		`<tspan class="tc f-up tb">FIRING</tspan><tspan class="tc f-up"> LOG</tspan>`,
		`<tspan class="tb">bold</tspan><tspan class="ts"> and </tspan><tspan class="tm">code</tspan>`,
		`<tspan class="ti">an </tspan><tspan class="ti tb">aside</tspan><tspan class="ti"> in </tspan><tspan class="ti tm">code</tspan>`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("the swimlane lacks %s", want)
		}
	}
}
