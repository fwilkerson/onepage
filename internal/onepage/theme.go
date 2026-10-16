package onepage

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var contract = []string{"canvas", "surface", "surface-raised", "surface-sunken", "line", "line-strong", "ink", "ink-2", "font-sans", "font-mono"}

type theme struct {
	path    string
	css     string
	tokens  map[string]string
	colours []string
}

var (
	cssComment  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssToken    = regexp.MustCompile(`(^|[^\w-])--([\w-]+)\s*:\s*([^;}]*)`)
	cssVar      = regexp.MustCompile(`(^|[^\w-])--([\w-]+)`)
	cssURL      = regexp.MustCompile(`url\(\s*(?:"([^"]*)"|'([^']*)'|([^)"'\s]*))\s*\)`)
	colourValue = regexp.MustCompile(`^(?:#[0-9a-fA-F]{3,8}|(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\(.*\))$`)
	aliasValue  = regexp.MustCompile(`^var\(\s*--([\w-]+)\s*\)$`)
)

var fontTypes = map[string]string{
	".woff2": "font/woff2", ".woff": "font/woff", ".ttf": "font/ttf", ".otf": "font/otf",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
}

func loadTheme(file string) (*theme, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoTheme, err)
	}
	css := cssComment.ReplaceAllString(string(b), "")
	th := &theme{path: file, tokens: map[string]string{}}
	var order []string
	for _, m := range cssToken.FindAllStringSubmatch(css, -1) {
		if _, seen := th.tokens[m[2]]; !seen {
			order = append(order, m[2])
		}
		th.tokens[m[2]] = strings.TrimSpace(m[3])
	}
	var missing []string
	for _, name := range contract {
		if _, ok := th.tokens[name]; !ok {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: the theme defines no %s", ErrMissingToken, strings.Join(missing, ", "))
	}
	for _, name := range order {
		if th.isColour(name, 0) {
			th.colours = append(th.colours, name)
		}
	}

	css = cssVar.ReplaceAllString(css, "${1}--theme-${2}")
	var errs []string
	css = cssURL.ReplaceAllStringFunc(css, func(u string) string {
		m := cssURL.FindStringSubmatch(u)
		ref := m[1] + m[2] + m[3]
		if ref == "" || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "#") {
			return u
		}
		at, _, _ := strings.Cut(ref, "?")
		at, _, _ = strings.Cut(at, "#")
		if !filepath.IsAbs(at) {
			at = filepath.Join(filepath.Dir(file), at)
		}
		b, err := os.ReadFile(at)
		if err != nil {
			errs = append(errs, err.Error())
			return u
		}
		kind, ok := fontTypes[strings.ToLower(filepath.Ext(at))]
		if !ok {
			kind = "application/octet-stream"
		}
		return fmt.Sprintf(`url("data:%s;base64,%s")`, kind, base64.StdEncoding.EncodeToString(b))
	})
	if len(errs) > 0 {
		return nil, fmt.Errorf("%w: a file it names: %s", ErrNoTheme, strings.Join(errs, "; "))
	}
	th.css = strings.TrimSpace(css)
	return th, nil
}

func (th *theme) isColour(name string, depth int) bool {
	v := th.tokens[name]
	if m := aliasValue.FindStringSubmatch(v); m != nil && depth < 8 {
		return th.isColour(m[1], depth+1)
	}
	return colourValue.MatchString(v)
}

func (th *theme) has(name string) bool { return slices.Contains(th.colours, name) }

func (th *theme) colour(name, otherwise string) string {
	if th.has(name) {
		return "var(--theme-" + name + ")"
	}
	return otherwise
}
