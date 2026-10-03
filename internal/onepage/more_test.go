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

const moreJS = `
const panel = document.getElementById("more");
const state = () => panel.hidden ? "closed" :
  [...panel.children].map((c) => c.textContent.trim()).join(" | ") + (below ? " (" + below() + ")" : "");
let below = null;
const beside = (el) => () => {
  const e = el.getBoundingClientRect(), p = panel.getBoundingClientRect();
  return p.top >= e.bottom && p.top - e.bottom < 20 || p.bottom <= e.top && e.top - p.bottom < 20 ? "beside it" : "away from it";
};
const click = (el) => el.dispatchEvent(new MouseEvent("click", { bubbles: true }));
const key = (el, k) => el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true }));
const howto = document.querySelector('[data-more="4.2"]');
const box = document.querySelector('[data-more="pots"]');
const out = [];
const step = (label, act, el) => { below = el ? beside(el) : null; act(); out.push(label + ": " + state()); };

step("loaded", () => {});
step("click 4.2", () => click(howto), howto);
step("Escape", () => key(document.body, "Escape"));
step("click pots", () => click(box), box);
step("click inside", () => click(panel.querySelector("p")), box);
step("click outside", () => click(document.querySelector(".tblock")));
step("click pots twice", () => { click(box); click(box); });
step("Enter on 4.2", () => key(howto, "Enter"), howto);
step("click pots with 4.2 open", () => click(box), box);
step("print", () => dispatchEvent(new Event("beforeprint")));
out.push("lit after print: " + document.querySelectorAll(".open").length);

const print = [...document.styleSheets].flatMap((s) => [...s.cssRules])
  .filter((r) => r.media && r.media.mediaText === "print")
  .flatMap((r) => [...r.cssRules])
  .some((r) => panel.matches(r.selectorText) && r.style.display === "none");
out.push("print rule: " + (print ? "hidden" : "shown"));
document.body.append(Object.assign(document.createElement("pre"), { id: "report", textContent: out.join("\n") }));
`

const openJS = `
const panel = document.getElementById("more");
document.body.append(Object.assign(document.createElement("pre"), { id: "report",
  textContent: panel.hidden ? "closed" : [...panel.children].map((c) => c.textContent.trim()).join(" | ") }));
`

var moreReport = regexp.MustCompile(`(?s)<pre id="report">(.*?)</pre>`)

func TestClickOpensADetail(t *testing.T) {
	t.Parallel()
	chrome, err := shot.Find(shot.Browsers)
	if err != nil {
		t.Skip(err)
	}
	page, err := onepage.Render("kiln.txt", kilnWithABoxDetail(t))
	if err != nil {
		t.Fatal(err)
	}
	report := func(script, hash string) string {
		file := filepath.Join(t.TempDir(), "kiln.html")
		html := strings.Replace(string(page), "</body>", "<script>"+script+"</script>\n</body>", 1)
		if err := os.WriteFile(file, []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
		url, err := shot.Target(file)
		if err != nil {
			t.Fatal(err)
		}
		dom, err := shot.DOM(t.Context(), chrome, url+hash, 1600, 900, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		m := moreReport.FindSubmatch(dom)
		if m == nil {
			t.Fatalf("the script wrote no report")
		}
		return string(m[1])
	}

	howto := "Read the log | The log keeps each firing. | It is never pruned."
	pots := "a firing | Each firing cools for a day."
	want := strings.Join([]string{
		"loaded: closed",
		"click 4.2: " + howto + " (beside it)",
		"Escape: closed",
		"click pots: " + pots + " (beside it)",
		"click inside: " + pots + " (beside it)",
		"click outside: closed",
		"click pots twice: closed",
		"Enter on 4.2: " + howto + " (beside it)",
		"click pots with 4.2 open: " + pots + " (beside it)",
		"print: closed",
		"lit after print: 0",
		"print rule: hidden",
	}, "\n")
	if got := report(moreJS, ""); got != want {
		t.Errorf("the details went\n%s\nwant\n%s", got, want)
	}
	if got := report(openJS, "#more-4.2"); got != howto {
		t.Errorf("opened at #more-4.2, the page shows %q, want %q", got, howto)
	}
}
