package onepage

import (
	_ "embed"
	"errors"
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

var palette = map[string]colour{
	"amber":  {"#cf6a08", "#ffb14e", "8%"},
	"cyan":   {"#0f72c8", "#5ccdfb", "6%"},
	"green":  {"#07896a", "#5fe6a6", "8%"},
	"violet": {"#7d45c9", "#d6a2ff", "7%"},
}

func Render(path string, src []byte) ([]byte, error) {
	d, errs := parse(path, src)
	r := &renderer{doc: d, path: path}
	body := r.body()
	errs = append(errs, r.errs...)
	if len(errs) > 0 {
		return nil, joinByLine(errs)
	}

	type actorView struct{ Name, Paper, Dark, Lane string }
	data := struct {
		Title, Source, Body, Diamond string
		Actors                       []actorView
	}{Title: esc(d.title), Source: path, Body: body, Diamond: "ink"}
	for _, a := range d.actors {
		c := palette[a.colour]
		data.Actors = append(data.Actors, actorView{a.name, c.paper, c.dark, c.lane})
	}
	for _, m := range d.marks {
		if m.kind == "diamond" && m.actor != "" {
			data.Diamond = m.actor
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
