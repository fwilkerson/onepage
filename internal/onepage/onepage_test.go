package onepage_test

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/fwilkerson/onepage/internal/onepage"
)

func renderKiln(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("testdata/kiln.txt")
	if err != nil {
		t.Fatal(err)
	}
	page, err := onepage.Render("testdata/kiln.txt", src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(page)
}

func TestRenderDrawsEveryBlock(t *testing.T) {
	t.Parallel()
	page := renderKiln(t)

	for _, want := range []string{
		`<title>Kiln, on one sheet</title>`,
		"    --maker: #cf6a08;",
		"      --maker: #ffb14e;",
		`svg .bx.oven { stroke: var(--oven); fill: color-mix(in srgb, var(--oven) var(--tint), var(--cell)); }`,
		`<marker id="a-shelf" `,
		`.dia::before { content: ""; position: absolute; inset: 1px; background: var(--maker);`,
		`  span.planned { display: inline-block; border: 1px dashed currentColor;`,

		`<span class="lab">1 · Premise · sheet 1 of 1</span>`,
		`<div class="mark">kil<span class="log">n</span></div>`,
		`<div class="motto">Fire it once</div>`,
		`<p>A kiln, <code>kiln</code>, with a log.</p>`,
		`<p class="lead">Pots crack:</p>`,
		"    <div class=\"plist\">\n" +
			`      <b>P1</b><span>Nobody knows the last firing.</span><span class="see"><span class="bal log" data-item="1">1</span> <span class="flag" data-n="1" data-note="1"></span></span>` + "\n" +
			`      <b>P2</b><span>Shelves overflow: <code>kiln ls | wc</code>.</span><span class="see"><span class="bal shelf" data-item="2">2</span></span>` + "\n" +
			"    </div>",
		`<p class="big">The <span class="shelf">store room</span> keeps pots; <span class="log">the log</span> keeps firings.</p>`,
		`<p><b>What happened:</b> each firing.</p>`,
		"    <ol>\n" +
			`      <li><span class="dia" data-n="1"></span><span><b>Loading:</b> what goes in.</span></li>` + "\n" +
			"    </ol>",
		`<span class="maker"><i></i><b>MAKER</b><em>the potter</em></span>`,
		`<span class="sym"><svg width="30" height="10"><line x1="1" y1="5" x2="28" y2="5" class="w" marker-end="url(#a)"/></svg>work moves</span>`,
		`<span class="sym"><svg width="30" height="10"><line x1="1" y1="5" x2="29" y2="5" class="d log"/></svg>reads or writes the log</span>`,
		`<span class="sym"><svg width="36" height="10"><line x1="1" y1="5" x2="35" y2="5" class="ph log"/></svg>planned</span>`,
		`<span class="sym"><span class="bal">1</span>part</span>`,
		`<span class="sym"><span class="flag" data-n="1"></span>asked of a studio</span>`,
		`<span class="sym"><span class="dia" data-n="1"></span>your attention</span>`,

		`<h2 class="sh"><span class="n">2</span><span class="t">WORKFLOW<small>one firing</small></span><span class="rule"></span><span class="note">a lane for each actor</span></h2>`,
		"\n<svg width=\"10\" height=\"10\"><rect class=\"bx oven\"/></svg>\n",

		`<span class="lab"><span class="tag">A</span>Detail A — when a pot cracks<span class="rule-note"><b>the log refuses</b> a guess.</span></span>`,
		`<svg width="34" height="97" viewBox="0 0 34 97" style="grid-row: 1 / 3">`,
		`<path d="M6,0 V97 M28,0 V97 M6,23 H28 M6,72 H28" class="w"/>`,
		`<path d="M17,89 V8" class="w dash" marker-end="url(#a)"/>`,
		`<div class="rung log planned"><b class="log">2 · Alarm <span class="planned">planned</span><small>the log's own bell</small></b><span>It rings.</span></div>` + "\n" +
			`      <div class="rung oven"><b>1 · Cool<small>the kiln itself</small></b><span>It cools slowly.</span></div>`,
		`<span class="lab">General notes — what the kiln asks of a studio</span>`,
		`<li data-note="1"><span class="flag" data-n="1"></span><span><code>vent</code> stays open. <span class="where">Always.</span></span></li>`,
		`<p class="flags-note">Each flag marks where its note applies.</p>`,

		"<div class=\"system\">\n<svg class=\"assembly\" width=\"960\" ",
		`<div class="ttl">PARTS LIST<span>what the kiln is made of</span></div>`,
		`<tr data-item="1"><td><span class="bal log">1</span></td><td><b>The log</b> <span>one file per kiln</span></td><td>Firings outlive the potter.</td></tr>`,
		`<tr data-item="2"><td><span class="bal shelf">2</span></td><td><b>The <code>shelf</code></b> <span class="planned shelf">planned</span> <span>a rack</span></td><td>Pots wait.</td></tr>`,

		`<div class="hdr"><span class="no">4.1</span><span class="to">Fire a load</span><span class="ty"><code class="maker">kiln fire</code><em>or ask</em><code class="oven">helper</code></span></div>`,
		`<p class="cap">It fires <b>once</b>.</p>`,
		`<div class="inline"><pre class="term"><span class="p">$</span> <span class="c">kiln fire -cone 6</span></pre><p class="cap">Beside the terminal.</p></div>`,
		`<code class="maker">kiln log &lt;id&gt;</code>`,
		"\n<pre class=\"term\"><span class=\"p\">$</span> <span class=\"c\">kiln log a88</span>\n" +
			`<span class="k">firing</span> <span class="i">a88</span>  <span class="ok">done</span>  cone 6` + "\n" +
			`  <span class="k">note</span> <span class="h">4e72a67</span> &lt;hot&gt; &amp; {x}` + "\n" +
			`       cracked <span class="q">Why?</span> <span class="a">a)</span></pre>`,

		`<div class="item" data-more="4.2" tabindex="0">`,
		`<span class="to">Read the log</span><span class="more-mk"></span>`,
		`<span class="sym"><span class="more-mk"></span>click for more</span>`,
		"<div class=\"mores\" hidden>\n" +
			`<section id="more-4.2"><h3>Read the log</h3><p>The log <b>keeps</b> each firing.</p><p>It is never pruned.</p></section>` + "\n" +
			"</div>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks\n%s", want)
		}
	}
	if strings.Contains(page, "\n  .planned {") {
		t.Errorf("page styles every element marked planned as the badge, a rung included")
	}
	if n := strings.Count(page, `<div class="col">`); n != 2 {
		t.Errorf("page has %d how-to columns, want 2", n)
	}
}

func kilnWithABoxDetail(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/kiln.txt")
	if err != nil {
		t.Fatal(err)
	}
	const pots = "| ×N, one by one ▲1\n"
	if n := strings.Count(string(b), pots); n != 1 {
		t.Fatalf("kiln.txt holds %q %d times, want once", pots, n)
	}
	return []byte(strings.Replace(string(b), pots, pots+"more Each firing cools for a day.\n", 1))
}

func TestRenderMarksABoxThatOpens(t *testing.T) {
	t.Parallel()
	page, err := onepage.Render("kiln.txt", kilnWithABoxDetail(t))
	if err != nil {
		t.Fatal(err)
	}
	box := regexp.MustCompile(`(?s)<g class="opens" data-more="pots" tabindex="0">\n(.*?)\n  </g>\n`).FindStringSubmatch(string(page))
	if box == nil {
		t.Fatalf("page draws no group opening pots' detail")
	}
	for _, want := range []string{`class="bx oven"`, `class="tb">a firing</text>`, `<g class="more-mk" transform="translate(`} {
		if !strings.Contains(box[1], want) {
			t.Errorf("pots' group lacks %s:\n%s", want, box[1])
		}
	}
	if want := `<section id="more-pots"><h3><b>a firing</b></h3><p>Each firing cools for a day.</p></section>`; !strings.Contains(string(page), want) {
		t.Errorf("page lacks\n%s", want)
	}
}

func TestRenderKeepsTheSheetsBehaviour(t *testing.T) {
	t.Parallel()
	page := renderKiln(t)

	for _, want := range []string{
		"function fit()",
		`if (e.key === "f" || e.key === "F") { fitPage = !fitPage; fit(); }`,
		`if (e.key === "p" || e.key === "P") document.documentElement.classList.toggle("blueprint");`,
		`for (const key of ["item", "note"])`,
		`<span class="bal log" data-item="1">1</span>`,
		`<tr data-item="1">`,
		`<span class="flag" data-n="1" data-note="1"></span>`,
		`<li data-note="1">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if m := regexp.MustCompile(`(?:src|href)=|url\((?:["']?)(?:https?:)?//|@import`).FindString(page); m != "" {
		t.Errorf("page fetches from elsewhere: %s", m)
	}
}

func TestRenderRefusesAnUnknownName(t *testing.T) {
	t.Parallel()
	const ok = "actor you amber the developer\n" +
		"::: assembly\npart 1 @you | A | b | c\n:::\n" +
		"::: details\nnotes | N\nnote 1 | n\n:::\n"
	const lane = "::: swimlane\nlane @you\nphase 1 | P\nbox a @you 1 | A\n"
	const asm = "::: assembly\nbox a @you A1 | A\n"
	const attn = "::: titleblock\ncell attn | A\nitem ◆1 | one\n:::\n"

	for _, c := range []struct {
		name string
		src  string
		err  error
		line int
	}{
		{"block", ok + "::: bogus\n:::\n", onepage.ErrUnknownBlock, 9},
		{"keyword", ok + "frobnicate x\n", onepage.ErrUnknownKeyword, 9},
		{"keyword in a block", ok + "::: titleblock\ncell id | x\nrung @you | a | b | c\n:::\n", onepage.ErrUnknownKeyword, 11},
		{"colour", ok + "actor them mauve they\n", onepage.ErrUnknownColour, 9},
		{"actor named for a theme variable", ok + "actor ink cyan ink\n", onepage.ErrReserved, 9},
		{"actor named for a theme class", ok + "actor planned cyan p\n", onepage.ErrReserved, 9},
		{"actor in text", ok + "::: titleblock\ncell id | x\npara see @nobody{this}\n:::\n", onepage.ErrUnknownActor, 11},
		{"actor on a part", ok + "::: assembly\npart 2 @nobody | A | b | c\n:::\n", onepage.ErrUnknownActor, 10},
		{"legend mark", ok + "mark star a star\n", onepage.ErrUnknownMark, 9},
		{"legend line", ok + "line wavy a wave\n", onepage.ErrUnknownMark, 9},
		{"terminal mark", ok + "::: spec\ncolumn\nhowto 1 | a | `b` @you\n```term\n{zz:x}\n```\n:::\n", onepage.ErrUnknownMark, 13},
		{"part", ok + "::: titleblock\ncell id | x\npara see {9}\n:::\n", onepage.ErrUnknownPart, 11},
		{"note", ok + "::: titleblock\ncell id | x\npara see ▲7\n:::\n", onepage.ErrUnknownNote, 11},
		{"diamond", ok + attn + "::: titleblock\ncell id | x\npara see ◆6\n:::\n", onepage.ErrUnknownDiamond, 15},
		{"fields", ok + "::: assembly\npart 2 @you | only one\n:::\n", onepage.ErrFields, 10},
		{"unclosed block", ok + "::: spec\ncolumn\n", onepage.ErrUnclosed, 9},
		{"unclosed fence", ok + "```svg\n<svg/>\n", onepage.ErrUnclosed, 9},
		{"arrow to no box", ok + lane + "arrow a nowhere\n:::\n", onepage.ErrUnknownBox, 13},
		{"arrow from no box", ok + lane + "arrow nowhere a\n:::\n", onepage.ErrUnknownBox, 13},
		{"framed box", ok + "::: details\ndetail A | T\n:::\n" + lane + "detail A | a nowhere\n:::\n", onepage.ErrUnknownBox, 16},
		{"frame letter", ok + lane + "detail Z | a\n:::\n", onepage.ErrUnknownDetail, 13},
		{"box used twice", ok + lane + "box a @you 1 | A again\n:::\n", onepage.ErrDuplicate, 13},
		{"lane used twice", ok + lane + "lane @you\n:::\n", onepage.ErrDuplicate, 13},
		{"phase used twice", ok + lane + "phase 1 | Q\n:::\n", onepage.ErrDuplicate, 13},
		{"box in no lane", ok + "::: swimlane\nphase 1 | P\nbox a @you 1 | A\n:::\n", onepage.ErrUnknownLane, 11},
		{"box in no phase", ok + "::: swimlane\nlane @you\nphase 1 | P\nbox a @you 2 | A\n:::\n", onepage.ErrUnknownPhase, 12},
		{"box's place", ok + "::: swimlane\nlane @you\nphase 1 | P\nbox a @you 1.2-1 | A\n:::\n", onepage.ErrFields, 12},
		{"box's variant", ok + "::: swimlane\nlane @you\nphase 1 | P\nbox a @you 1 round | A\n:::\n", onepage.ErrUnknownMark, 12},
		{"arrow's style", ok + lane + "box b @you 1 | B\narrow a b wavy\n:::\n", onepage.ErrUnknownMark, 14},
		{"text planned", ok + lane + "text @you 1 planned | T\n:::\n", onepage.ErrUnknownMark, 13},
		{"lane's actor", ok + "::: swimlane\nlane @nobody\n:::\n", onepage.ErrUnknownActor, 10},
		{"flag in a box", ok + lane + "box b @you 1 | B ▲7\n:::\n", onepage.ErrUnknownNote, 13},
		{"balloon in a box", ok + lane + "box b @you 1 | {9} B\n:::\n", onepage.ErrUnknownPart, 13},
		{"diamond in a box", ok + attn + lane + "box b @you 1 | B ◆6\n:::\n", onepage.ErrUnknownDiamond, 17},
		{"arrow through a box", ok + lane + "box b @you 1 | B\nbox c @you 1 | C\narrow a c\n:::\n", onepage.ErrNoWay, 15},
		{"too wide", ok + "::: swimlane\nlane @you\nphase 1 | P\nbox a @you 1 | " + strings.Repeat("wide ", 300) + "\n:::\n", onepage.ErrTooWide, 9},
		{"balloon in a part", ok + asm + "box b @you B1 | {9} B\n:::\n", onepage.ErrUnknownPart, 11},
		{"balloon in a group", ok + asm + "group g A1 | G {9}\n:::\n", onepage.ErrUnknownPart, 11},
		{"connection to no part", ok + asm + "arrow a nowhere\n:::\n", onepage.ErrUnknownBox, 11},
		{"connection from no part", ok + asm + "arrow nowhere a\n:::\n", onepage.ErrUnknownBox, 11},
		{"part used twice", ok + asm + "store a @you B1 | A\n:::\n", onepage.ErrDuplicate, 11},
		{"parts in one cell", ok + asm + "person b @you A1-B1 | B\n:::\n", onepage.ErrOverlap, 11},
		{"part across a group's edge", ok + asm + "group g B1-B2 | G\nbox c @you B1 | C\nbox b @you A2-B2 | B\n:::\n", onepage.ErrOverlap, 13},
		{"groups that overlap", ok + asm + "group g A1-B1 | G\ngroup h B1 | H\n:::\n", onepage.ErrOverlap, 12},
		{"part's place", ok + asm + "box b @you b1 | B\n:::\n", onepage.ErrFields, 11},
		{"part's variant", ok + asm + "box b @you B1 chip | B\n:::\n", onepage.ErrUnknownMark, 11},
		{"part with no actor", ok + asm + "box b B1 | B\n:::\n", onepage.ErrFields, 11},
		{"group with no part", ok + asm + "group g B1 | G\n:::\n", onepage.ErrUnknownBox, 11},
		{"group planned", ok + asm + "group g A1 planned | G\n:::\n", onepage.ErrUnknownMark, 11},
		{"group of two lines", ok + asm + "group g A1 | G | H\n:::\n", onepage.ErrFields, 11},
		{"connection's style", ok + asm + "box b @you B1 | B\narrow a b wavy\n:::\n", onepage.ErrUnknownMark, 12},
		{"no room to arrive straight", ok + "::: assembly\nbox a @you A1-A3 | A\nbox b @you B1 | B\nbox c @you B3 | C\narrow a b dotted\narrow a c dotted\n:::\n", onepage.ErrHeadOnTurn, 13},
		{"connection through its group", ok + asm + "group g A1-A2 | G\nbox b @you A2 | B\narrow g a\n:::\n", onepage.ErrNoWay, 13},
		{"drawing too wide", ok + "::: assembly\nbox a @you A1 | " + strings.Repeat("wide ", 300) + "\n:::\n", onepage.ErrTooWide, 9},
		{"drawing by hand in an assembly", ok + "::: assembly\n```svg\n<svg/>\n```\n:::\n", onepage.ErrUnknownBlock, 10},
		{"rail of two lines", ok + asm + "rail r @you B1-B2 | R | S\n:::\n", onepage.ErrFields, 11},
		{"rail with two ways out", ok + asm + "rail r @you B1-B2 | R\nbox c @you C1 | C\narrow r a\narrow r c\n:::\n", onepage.ErrFields, 14},
		{"valve with no actor", ok + asm + "valve v | {1}\n:::\n", onepage.ErrFields, 11},
		{"valve used twice", ok + asm + "valve a @you | {1}\n:::\n", onepage.ErrDuplicate, 11},
		{"connection via no valve", ok + asm + "box b @you B1 | B\narrow a b via nowhere\n:::\n", onepage.ErrUnknownBox, 12},
		{"connection via a part", ok + asm + "box b @you B1 | B\narrow a b via a\n:::\n", onepage.ErrUnknownBox, 12},
		{"connection's actor", ok + asm + "box b @you B1 | B\narrow a b @nobody\n:::\n", onepage.ErrUnknownActor, 12},
		{"balloon in a label", ok + asm + "box b @you B1 | B\narrow a b | see {9}\n:::\n", onepage.ErrUnknownPart, 12},
		{"balloon on an unused valve", ok + asm + "valve v @you | {9}\n:::\n", onepage.ErrUnknownPart, 11},
		{"rail's label too wide", ok + asm + "rail r @you B1 | " + strings.Repeat("WIDE ", 60) + "\n:::\n", onepage.ErrTooWide, 9},
		{"chip left open", ok + asm + "box b @you B1 | [F\n:::\n", onepage.ErrUnknownMark, 11},
		{"detail at the top", ok + "mark more m\nmore x\n", onepage.ErrUnknownKeyword, 10},
		{"detail after a text", ok + "mark more m\n" + lane + "text @you 1 | T\nmore x\n:::\n", onepage.ErrUnknownKeyword, 15},
		{"detail after an arrow", ok + "mark more m\n" + lane + "box b @you 1 | B\narrow a b\nmore x\n:::\n", onepage.ErrUnknownKeyword, 16},
		{"detail on a part", ok + "mark more m\n" + asm + "more x\n:::\n", onepage.ErrUnknownKeyword, 12},
		{"detail in the notes", ok + "mark more m\n::: details\nnotes | N\nmore x\n:::\n", onepage.ErrUnknownKeyword, 12},
		{"detail with no key", ok + lane + "more x\n:::\n", onepage.ErrUnkeyedMore, 13},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			page, err := onepage.Render("s.txt", []byte(c.src))
			if page != nil {
				t.Errorf("rendered a page from a refused source")
			}
			var se *onepage.SourceError
			if !errors.As(err, &se) {
				t.Fatalf("err = %v, want a SourceError", err)
			}
			if !errors.Is(err, c.err) || se.Line != c.line || se.Path != "s.txt" {
				t.Errorf("err = %v, want %v at s.txt:%d", err, c.err, c.line)
			}
		})
	}

	if _, err := onepage.Render("s.txt", []byte(ok)); err != nil {
		t.Errorf("the unbroken source: %v", err)
	}
	if _, err := onepage.Render("s.txt", []byte(ok+"::: titleblock\ncell id | x\npara see ◆1\n:::\n"+attn)); err != nil {
		t.Errorf("a diamond before its item: %v", err)
	}
}

func TestRenderReportsEveryProblem(t *testing.T) {
	t.Parallel()
	_, err := onepage.Render("s.txt", []byte("actor x mauve x\nfrobnicate\n"))
	for _, want := range []string{"s.txt:1: unknown colour", "s.txt:2: unknown keyword"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to hold %q", err, want)
		}
	}
}
