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

const dotted = "actor you amber the developer\n" +
	"actor agent cyan an agent\n" +
	"mark diamond @agent an agent's lean\n" +
	"mark dot @you waits on you\n" +
	"::: titleblock\ncell attn | A\nitem ◆1 | lean\nitem ●2 | wait\npara see ●2 and ◆1\n:::\n" +
	"::: swimlane\nlane @you\nlane @agent\nphase 1 | P\nbox a @you 1 | ●2 waits | asks ●2\nbox b @agent 1 | ◆1 leans | leans ◆1\n:::\n"

func TestRenderDrawsADotLikeADiamond(t *testing.T) {
	t.Parallel()
	page, err := onepage.Render("s.txt", []byte(dotted))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(page)
	for _, want := range []string{
		`.disc::before { content: ""; position: absolute; inset: 1.5px; border-radius: 50%; background: var(--you); }`,
		`svg .disc circle { fill: var(--you); }`,
		`.dia::before { content: ""; position: absolute; inset: 1px; background: var(--agent);`,
		`<span class="sym"><span class="disc" data-n="1"></span>waits on you</span>`,
		`<li><span class="disc" data-n="2"></span><span>wait</span></li>`,
		`<p>see <span class="disc" data-n="2"></span> and <span class="dia" data-n="1"></span></p>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("page lacks\n%s", want)
		}
	}
	dots := regexp.MustCompile(`<g class="disc" transform="translate\(\d+,\d+\)"><circle r="7"/><text y="3.5">2</text></g>`).FindAllString(s, -1)
	dias := regexp.MustCompile(`<g class="dia" transform="translate\(\d+,\d+\)">`).FindAllString(s, -1)
	if len(dots) != 2 || len(dias) != 2 {
		t.Errorf("the drawing has %d dots and %d diamonds, want 2 of each, one leading a line and one trailing it", len(dots), len(dias))
	}
}

const dotJS = `
const marks = [...document.querySelectorAll(".flag, .dia, .disc, .bal, .notes li, .bom tbody tr")];
const style = (el, pseudo) => { const c = getComputedStyle(el, pseudo); return c.backgroundColor + c.fill + c.color + c.filter; };
const look = (el) => el instanceof SVGElement ? style(el.querySelector("circle, path")) + style(el.querySelector("text")) :
  style(el) + style(el, "::before") + style(el, "::after");
const before = new Map(marks.map((el) => [el, look(el)]));
const lit = () => marks.filter((el) => look(el) !== before.get(el)).length;
const out = [];
for (const el of document.querySelectorAll(".dia, .disc")) {
  const name = (el.closest(".legend") ? "legend " : el.closest("svg") ? "drawing " : "text ") + (el.classList.contains("disc") ? "dot" : "diamond");
  el.dispatchEvent(new MouseEvent("mouseenter"));
  out.push(name + " lights " + lit());
  el.dispatchEvent(new MouseEvent("mouseleave"));
}
const dot = document.querySelector(".tblock .disc"), drawn = document.querySelector("svg .disc");
const shape = getComputedStyle(dot, "::before"), numeral = getComputedStyle(dot, "::after");
out.push("dot " + shape.borderRadius + " " + shape.backgroundColor + ", numeral " + numeral.color);
out.push("drawn dot " + getComputedStyle(drawn.querySelector("circle")).fill + ", numeral " + getComputedStyle(drawn.querySelector("text")).fill);
document.body.append(Object.assign(document.createElement("pre"), { id: "dots", textContent: [...new Set(out)].sort().join("\n") }));
`

func TestHoverTreatsADotAsADiamond(t *testing.T) {
	t.Parallel()
	chrome, err := shot.Find(shot.Browsers)
	if err != nil {
		t.Skip(err)
	}
	page, err := onepage.Render("s.txt", []byte(dotted))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "dots.html")
	html := strings.Replace(string(page), "</body>", "<script>"+dotJS+"</script>\n</body>", 1)
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
	m := regexp.MustCompile(`(?s)<pre id="dots">(.*?)</pre>`).FindSubmatch(dom)
	if m == nil {
		t.Fatalf("the script wrote no report")
	}
	want := strings.Join([]string{
		"dot 50% rgb(207, 106, 8), numeral rgb(26, 18, 6)",
		"drawing diamond lights 0",
		"drawing dot lights 0",
		"drawn dot rgb(207, 106, 8), numeral rgb(26, 18, 6)",
		"legend diamond lights 0",
		"legend dot lights 0",
		"text diamond lights 0",
		"text dot lights 0",
	}, "\n")
	if got := string(m[1]); got != want {
		t.Errorf("the dots went\n%s\nwant\n%s", got, want)
	}
}
