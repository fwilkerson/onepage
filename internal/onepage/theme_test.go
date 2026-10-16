package onepage_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/fwilkerson/onepage/internal/onepage"
)

const themed = "title Themed\n" +
	"theme theme/sheet.css\n" +
	"actor you you the developer\n" +
	"actor agent agent an agent session\n" +
	"actor repo ref the repository\n" +
	"actor eidos ink-2 eidos and its runner\n" +
	"::: spec\ncolumn\nhowto 1 | Ask | `eidos ask` @you or `eidos run` @agent\n```term\n$ eidos run E-1\nrun {i:r-1} {h:4e72a67} {ok:passed} {q:Why?} {a:a)}\n```\n:::\n" +
	"::: swimlane\nlane @you\nlane @agent\nphase 1 | ASK\nbox ask @you 1 | asks\nbox run @agent 1 | runs\n:::\n"

func TestRenderTakesItsLookFromATheme(t *testing.T) {
	t.Parallel()
	page, err := onepage.Render("testdata/themed.txt", []byte(themed))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(page)
	for _, want := range []string{
		`src: url("data:font/woff2;base64,d09GMmZha2U=") format("woff2");`,
		"  --theme-canvas: #101112;",
		"  --theme-ref: var(--theme-agent);",
		"    --sheet: var(--theme-canvas);",
		"    --line: var(--theme-line-strong);",
		"    --rule: var(--theme-line);",
		"    --tech: var(--theme-font-sans);",
		"    --hot: var(--theme-ink);",

		"    --you: var(--theme-you);",
		"    --agent: var(--theme-agent);",
		"    --repo: var(--theme-ref);",
		"    --eidos: var(--theme-ink-2);",
		`svg .bx.you { stroke: var(--you); fill: var(--theme-you-tint); }`,
		`svg .bx.agent { stroke: var(--agent); fill: var(--theme-surface-raised); }`,
		`.rung.you { border-color: var(--you); background: var(--theme-you-tint); }`,
		`.item .ty code.agent { border-color: var(--agent); background: transparent; }`,
		`.item .ty code.you { border-color: var(--you); background: var(--theme-you-tint); }`,
		`svg .lane.you { fill: color-mix(in srgb, var(--theme-you-tint) 45%, transparent); }`,
		`svg .lane.agent { fill: transparent; }`,
		`<rect x="0" y="30" width="5" height="`,

		`.bal.hot, .bom tr.hot > td:first-child > .bal { color: var(--cell); }`,
		`.term .k { color: var(--theme-syn-name); }`,
		`.term .h { color: var(--theme-ref); }`,
		`.term .ok { color: var(--theme-ink); }`,
		`.term .q { color: var(--theme-ink); }`,
		`.term .a { color: var(--theme-agent); }`,
		`print-color-adjust: exact;`,
		"Keys: F fits the whole sheet.\n",
		"Drawn in the theme\n  theme/sheet.css: one dark look",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("page lacks\n%s", want)
		}
	}
	for _, not := range []string{"fonts/Plain.woff2", "blueprint", "paper", "\n  --canvas:"} {
		if strings.Contains(s, not) {
			t.Errorf("a themed page holds %q", not)
		}
	}
	if m := regexp.MustCompile(`(?:src|href)=|url\((?:["']?)(?:https?:)?//|@import`).FindString(s); m != "" {
		t.Errorf("page fetches from elsewhere: %s", m)
	}
}

func TestRenderFindsAThemeByAnAbsolutePath(t *testing.T) {
	t.Parallel()
	abs, err := filepath.Abs("testdata/theme/sheet.css")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Replace(themed, "theme theme/sheet.css", "theme "+abs, 1)
	page, err := onepage.Render(filepath.Join(t.TempDir(), "s.txt"), []byte(src))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if want := "d09GMmZha2U="; !strings.Contains(string(page), want) {
		t.Errorf("page lacks the font %s", want)
	}
}

func TestRenderRefusesATheme(t *testing.T) {
	t.Parallel()
	css, err := os.ReadFile("testdata/theme/sheet.css")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	short := regexp.MustCompile(`(?m)^  --(surface-sunken|font-mono):.*\n`).ReplaceAllString(string(css), "")
	write("short.css", short)
	write("lost.css", strings.Replace(string(css), "fonts/Plain.woff2", "fonts/Lost.woff2", 1))

	for _, c := range []struct {
		name, src string
		err       error
		line      int
		says      []string
	}{
		{"missing tokens", "title T\ntheme short.css\n", onepage.ErrMissingToken, 2, []string{"--surface-sunken, --font-mono"}},
		{"missing stylesheet", "theme nowhere.css\n", onepage.ErrNoTheme, 1, []string{"nowhere.css"}},
		{"missing font", "theme lost.css\n", onepage.ErrNoTheme, 1, []string{"Lost.woff2"}},
		{"theme twice", "theme " + mustAbs(t, "testdata/theme/sheet.css") + "\ntheme short.css\n", onepage.ErrDuplicate, 2, []string{"theme, first on line 1"}},
		{"palette colour with a theme", "actor them amber they\ntheme " + mustAbs(t, "testdata/theme/sheet.css") + "\n", onepage.ErrUnknownColour, 1,
			[]string{`"amber": the theme's colours are canvas, surface, surface-raised, surface-sunken, line, line-strong, ink, ink-2, you, you-tint, agent, ref, syn-name`}},
		{"actor named for the theme", "actor theme-ink ink x\n", onepage.ErrReserved, 1, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			page, err := onepage.Render(filepath.Join(dir, "s.txt"), []byte(c.src))
			if page != nil {
				t.Errorf("rendered a page from a refused source")
			}
			var se *onepage.SourceError
			if !errors.As(err, &se) || !errors.Is(err, c.err) || se.Line != c.line {
				t.Fatalf("err = %v, want %v on line %d", err, c.err, c.line)
			}
			for _, want := range c.says {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to say %s", err, want)
				}
			}
		})
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestRenderWithoutAThemeIsUnchanged(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"kiln", "memo"} {
		src, err := os.ReadFile("testdata/" + name + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile("testdata/" + name + ".html")
		if err != nil {
			t.Fatal(err)
		}
		page, err := onepage.Render("testdata/"+name+".txt", src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(page) != string(want) {
			t.Errorf("%s renders differently from testdata/%[1]s.html, the page as it was before themes", name)
		}
	}
}

func TestRenderMeasuresTheThemesFaces(t *testing.T) {
	t.Parallel()
	const sheet = "actor %s\n::: assembly\ngroup g A1 | A RUN · A FRESH SESSION PER JOB\nbox a @you A1 | a part\n:::\n"
	tab := regexp.MustCompile(`width="(\d+)" height="16" class="tab"`)
	width := func(src string) int {
		t.Helper()
		page, err := onepage.Render("testdata/s.txt", []byte(src))
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		m := tab.FindSubmatch(page)
		if m == nil {
			t.Fatalf("page draws no tab")
		}
		n, err := strconv.Atoi(string(m[1]))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	paper := width(strings.Replace(sheet, "%s", "you amber the developer", 1))
	themed := width("theme theme/sheet.css\n" + strings.Replace(sheet, "%s", "you you the developer", 1))
	if themed*100 < paper*115 {
		t.Errorf("a themed tab of capitals is %dpx, the paper's %dpx; the theme's sans sets capitals a fifth wider than the condensed face", themed, paper)
	}
}
