package onepage_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fwilkerson/onepage/internal/onepage"
	"github.com/fwilkerson/onepage/internal/shot"
)

const hoverJS = `
const marks = [...document.querySelectorAll(".flag, .dia, .bal, .notes li, .bom tbody tr")];
const style = (el, pseudo) => { const c = getComputedStyle(el, pseudo); return c.backgroundColor + c.fill + c.color + c.filter; };
const look = (el) =>
  el.matches("tr") ? style(el.cells[0]) :
  el instanceof SVGElement ? style(el.querySelector("circle, path")) + style(el.querySelector("text")) :
  style(el) + style(el, "::before") + style(el, "::after");
const kind = (el) =>
  el.matches("li") ? "note" : el.matches("tr") ? "part" :
  el.classList.contains("flag") ? "flag" : el.classList.contains("dia") ? "diamond" : "balloon";
const where = (el) =>
  el.matches("li, tr") ? "" :
  el.closest(".legend") ? "legend " : el.closest("svg") ? "drawing " :
  el.parentElement.matches(".notes li") ? "note's " : el.parentElement.matches("td:first-child") ? "part's " : "text ";
const name = (el) => where(el) + kind(el) + " " + (el.dataset.note ?? el.dataset.item ?? el.dataset.n ?? el.textContent.trim());
const before = new Map(marks.map((el) => [el, look(el)]));
const lit = () => marks.filter((el) => look(el) !== before.get(el)).map(name).sort().join(", ");
const out = [];
for (const [label, el] of [
  ["text flag 1", document.querySelector('.plist .flag[data-n="1"]')],
  ["drawing flag 1", [...document.querySelectorAll("svg .flag")].find((g) => g.textContent === "1")],
  ["note 2", document.querySelectorAll(".notes li")[1]],
  ["part 2", document.querySelector('.bom tr[data-item="2"]')],
  ["text balloon 1", document.querySelector('.plist .bal[data-item="1"]')],
]) {
  el.dispatchEvent(new MouseEvent("mouseenter"));
  out.push(label + ": " + lit());
  el.dispatchEvent(new MouseEvent("mouseleave"));
  out.push(label + " left: " + lit());
}
document.body.append(Object.assign(document.createElement("pre"), { id: "hover", textContent: out.join("\n") }));
`

var hoverReport = regexp.MustCompile(`(?s)<pre id="hover">(.*?)</pre>`)

func TestHoverLightsANotesMarks(t *testing.T) {
	t.Parallel()
	chrome, err := shot.Find(shot.Browsers)
	if err != nil {
		t.Skip(err)
	}
	b, err := os.ReadFile("testdata/kiln.txt")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, r := range [][2]string{
		{"| {2}\n", "| {2} ▲2\n"},
		{"| Always.\n", "| Always.\nnote 2 | Mind ▲1 as well. | Often.\n"},
		{"| Pots wait.\n", "| Pots wait for {1}.\n"},
	} {
		if n := strings.Count(src, r[0]); n != 1 {
			t.Fatalf("kiln.txt holds %q %d times, want once", r[0], n)
		}
		src = strings.Replace(src, r[0], r[1], 1)
	}
	page, err := onepage.Render("kiln.txt", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "kiln.html")
	html := strings.Replace(string(page), "</body>", "<script>"+hoverJS+"</script>\n</body>", 1)
	if err := os.WriteFile(file, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	url, err := shot.Target(file)
	if err != nil {
		t.Fatal(err)
	}
	dom, err := shot.DOM(t.Context(), chrome, url, 1600, 900, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	m := hoverReport.FindSubmatch(dom)
	if m == nil {
		t.Fatalf("the hover script wrote no report")
	}

	flag1 := "drawing flag 1, drawing flag 1, drawing flag 1, note 1, note's flag 1, text flag 1, text flag 1"
	want := strings.Join([]string{
		"text flag 1: " + flag1,
		"text flag 1 left: ",
		"drawing flag 1: " + flag1,
		"drawing flag 1 left: ",
		"note 2: note 2, note's flag 2, text flag 2",
		"note 2 left: ",
		"part 2: drawing balloon 2, part 2, part's balloon 2, text balloon 2",
		"part 2 left: ",
		"text balloon 1: drawing balloon 1, drawing balloon 1, part 1, part's balloon 1, text balloon 1, text balloon 1",
		"text balloon 1 left: ",
	}, "\n")
	if got := string(m[1]); got != want {
		t.Errorf("hovering lit\n%s\nwant\n%s", got, want)
	}
}
