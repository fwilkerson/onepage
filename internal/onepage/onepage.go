package onepage

import (
	"cmp"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

//go:embed page.tmpl
var pageTmpl string

var page = template.Must(template.New("page").Parse(pageTmpl))

var reserved = func() map[string]bool {
	css, _, _ := strings.Cut(pageTmpl, "</style>")
	m := map[string]bool{}
	for _, name := range regexp.MustCompile(`(?:\.|--)([a-z][a-z0-9-]*)`).FindAllStringSubmatch(css, -1) {
		m[name[1]] = true
	}
	return m
}()

type colour struct{ paper, dark, lane string }

var paperFaces = faces{mono: 11.5, label: 17, caps: 12, body: 12, small: 11.5, top: 16, pitch: 15, aside: 14, heading: 17, labelPitch: 14, dotted: 2}

// textWidth estimates the paper's faces. Measured in Chrome over every phrase of four sheets, Atkinson Hyperlegible
// Next sets capitals up to a fifth wider than the condensed tech face it stands in for, and its body text about 5%
// narrower than the system sans; sans and tech keep a themed sheet's worst case where the paper's is. A themed sheet is
// read on a dark screen, so it sets a size or so larger throughout.
var themeFaces = faces{sans: -0.055, tech: 0.2, mono: 12.5, label: 18, caps: 13, body: 13.5, small: 13,
	top: 18, pitch: 17, aside: 16, heading: 19, labelPitch: 16, dotted: 2.2}

var palette = map[string]colour{
	"amber":  {"#cf6a08", "#ffb14e", "8%"},
	"cyan":   {"#0f72c8", "#5ccdfb", "6%"},
	"green":  {"#07896a", "#5fe6a6", "8%"},
	"violet": {"#7d45c9", "#d6a2ff", "7%"},
}

func Render(path string, src []byte) ([]byte, error) {
	d, errs := parse(path, src)
	fail := func(line int, err error) { errs = append(errs, &SourceError{Path: path, Line: line, Err: err}) }
	var th *theme
	if d.theme.s != "" {
		file := d.theme.s
		if !filepath.IsAbs(file) {
			file = filepath.Join(filepath.Dir(path), file)
		}
		var err error
		if th, err = loadTheme(file); err != nil {
			fail(d.theme.line, err)
		}
	}
	for _, a := range d.actors {
		switch _, ok := palette[a.colour]; {
		case th != nil && !th.has(a.colour):
			fail(a.line, fmt.Errorf("%w %q: the theme's colours are %s", ErrUnknownColour, a.colour, strings.Join(th.colours, ", ")))
		case d.theme.s == "" && !ok:
			fail(a.line, fmt.Errorf("%w %q: the palette has amber, cyan, green and violet", ErrUnknownColour, a.colour))
		}
	}
	r := &renderer{doc: d, path: path, faces: paperFaces}
	if th != nil {
		r.faces, r.themed = themeFaces, true
	}
	body := r.body()
	errs = append(errs, r.errs...)
	if len(errs) > 0 {
		return nil, joinByLine(errs)
	}

	type actorView struct{ Name, Paper, Dark, Fill, Tag, Lane string }
	type themeView struct {
		Path, CSS, Key, Hash, OK, Question, Option string
		Grey                                       []string
	}
	data := struct {
		Title, Source, Body, Diamond, Dot, Flag string
		Actors                                  []actorView
		Theme                                   *themeView
	}{Title: esc(d.title), Source: path, Body: body, Diamond: "ink"}
	for _, a := range d.actors {
		if th == nil {
			c, v := palette[a.colour], "var(--"+a.name+")"
			data.Actors = append(data.Actors, actorView{a.name, c.paper, c.dark,
				"color-mix(in srgb, " + v + " var(--tint), var(--cell))",
				"color-mix(in srgb, " + v + " var(--tint), transparent)",
				"color-mix(in srgb, " + v + " " + c.lane + ", transparent)"})
			continue
		}
		c, tint, lane := th.colour(a.colour, ""), th.colour(a.colour+"-tint", ""), "transparent"
		if tint != "" {
			lane = "color-mix(in srgb, " + tint + " 45%, transparent)"
		}
		data.Actors = append(data.Actors, actorView{a.name, c, c, cmp.Or(tint, "var(--theme-surface-raised)"), cmp.Or(tint, "transparent"), lane})
	}
	if th != nil {
		ink := "var(--theme-ink)"
		data.Theme = &themeView{d.theme.s, th.css, th.colour("syn-name", ink), th.colour("ref", ink), th.colour("ok", ink), th.colour("you-ink", ink), th.colour("agent", ink), nil}
		for _, a := range d.actors {
			if a.colour == "ink-2" {
				data.Theme.Grey = append(data.Theme.Grey, a.name)
			}
		}
	}
	if len(d.dots) > 0 {
		data.Dot = "ink"
	}
	for _, m := range d.marks {
		switch {
		case m.kind == "diamond" && m.actor != "":
			data.Diamond = m.actor
		case m.kind == "dot":
			data.Dot = cmp.Or(m.actor, data.Dot, "ink")
		case m.kind == "flag":
			data.Flag = m.actor
		}
	}
	var b strings.Builder
	if err := page.Execute(&b, data); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func joinByLine(errs []error) error {
	slices.SortStableFunc(errs, func(a, b error) int {
		var ea, eb *SourceError
		errors.As(a, &ea)
		errors.As(b, &eb)
		return ea.Line - eb.Line
	})
	return errors.Join(errs...)
}
