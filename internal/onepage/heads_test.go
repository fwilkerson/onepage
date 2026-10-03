package onepage

import (
	"regexp"
	"strconv"
	"testing"
)

func TestHeadsAreAsThePageDrawsThem(t *testing.T) {
	t.Parallel()
	attr := func(re string) float64 {
		t.Helper()
		m := regexp.MustCompile(re).FindStringSubmatch(pageTmpl)
		if m == nil {
			t.Fatalf("page.tmpl lacks %s", re)
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got := attr(`<marker id="a-\{\{\.Name\}\}" viewBox="0 0 10 10" refX="([\d.]+)"`); got != 9 {
		t.Errorf("a head's refX is %v, want 9", got)
	}
	if got := attr(`<marker id="a-heavy-\{\{\.Name\}\}" viewBox="0 0 10 10" refX="([\d.]+)"`); got != 9 {
		t.Errorf("a heavy head's refX is %v, want 9", got)
	}
	if got := attr(`<marker id="a-\{\{\.Name\}\}"[^>]* markerWidth="([\d.]+)"`); got != headStrokes {
		t.Errorf("a head is %v strokes long, the renderer keeps room for %v", got, headStrokes)
	}
	if got := attr(`<marker id="a-heavy-\{\{\.Name\}\}"[^>]* markerUnits="userSpaceOnUse" markerWidth="([\d.]+)"`); got != heavyHead {
		t.Errorf("a heavy head is %vpx long, the renderer keeps room for %v", got, heavyHead)
	}
	for class, w := range strokes {
		if got := attr(`svg \.` + class + ` \{[^}]*stroke-width: ([\d.]+);`); got != w {
			t.Errorf("a .%s line's stroke is %v wide, the renderer keeps room for %v", class, got, w)
		}
	}
}
