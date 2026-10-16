# The one-page markup

A source is read a line at a time. Outside a fence, a blank line and a line starting with `#` are skipped, and every
other line starts with a keyword. A line with several fields splits them at each `|` with a space on either side. Every
word, actor and colour belongs to the source; the look, the keys, the hover and the panels belong to the renderer.

## Declarations

At the top level:

````
title <text>                         the page's title
theme <path>                         at most once: a stylesheet the page
                                     takes its look from, its path from
                                     the source's folder or absolute
actor <name> <colour> <text>         who acts: a lowercase name, a colour
                                     (amber, cyan, green, violet, or with
                                     a theme one of its colours) and what
                                     the legend says of it; a name the
                                     page's CSS uses, such as ink, or
                                     one starting theme- is refused
line arrow|dotted|planned [@a] <text>  a line style the legend explains
mark balloon|flag|diamond|dot|more [@a] <text>  a mark the legend
                                     explains; the diamond, the dot and
                                     the flag take their actor's colour,
                                     a flag of none is ink; more, the
                                     boxed plus on an element that opens
                                     a detail, is required once any more
                                     is written
view <n> <TITLE> | <subtitle> | <note> a numbered view's heading
```svg ... ```                       a drawing, kept as written
````

## Blocks

Blocks open with `::: <block>` and close with a line of `:::`.

````
::: titleblock                       the premise, closed by the legend
cell id|problem|answer|attn | <label>
wordmark <text>   motto <text>   para <text>   lead <text>   big <text>
point <label> | <text> | <marks>     a row of the problem list
item <mark> | <text>                 a numbered line, such as ◆1 or ●1

::: details                          panels side by side
detail <letter> | <title> | <note>   a lettered detail, then its ladder:
rung @<actor> [planned] | <title> | <who> | <text>
                                     listed from the first rung up
notes | <title>                      the general notes, each flagged:
note <n> | <text> | <where>
foot <text>

::: assembly                         a drawing beside its parts list, its
                                     parts placed and connections routed
box <name> @<actor> <cells> [stack] [planned] | <line> | ...
store <name> @<actor> <cells> [planned] | <line> | ...   a cylinder
person <name> @<actor> <cells> [planned] | <line> | ...  a figure
rail <name> @<actor> <cells> | <LABEL>   a heavy line down its cells,
                                     such as a main branch
group <name> [@<actor>] <cells> | <line>   a dash-dot edge around the
                                     parts within its cells, and a tab
valve <name> @<actor> | <marks>      a gate drawn on each arrow via it
arrow <part> <part> [dotted|planned] [@<actor>] [via <valve>] | <label> | ...
                                     in the colour of the part it leaves,
                                     or of the actor; a group may be
                                     either end; its label lines are drawn
                                     beside it
parts <TITLE> | <subtitle>
part <n> @<actor> [planned] | <name> | <what> | <why>

::: spec                             how-tos, in columns
column
howto <n> | <to> | <typed>           typed: `command` @actor, and words
caption <text>
```term [beside] ... ```             a terminal; beside puts the next
                                     caption beside it
more <text>                          a paragraph of the how-to's detail,
                                     opened by its number

::: swimlane                         a lane for each actor, phases left to
                                     right, boxes placed and arrows routed
lane @<actor> | <line> | ...         its lines under the lane's name, or
                                     the actor's label when none
phase <n> | <TITLE>                  drawn in the order written
box <name> @<lane> <place> [chip|note|stack] [planned] | <line> | ...
more <text>                          right after a box, a paragraph of
                                     its detail, opened by its name and
                                     marked on its top right corner
text @<lane> <place> | <line> | ...  italic words with no box
arrow <box> <box> [dotted|planned]   in the colour of the lane it leaves
detail <letter> | <box> <box> ...    a dashed frame, lettered for a detail;
                                     solid under a theme
````

## Places and cells

In a swimlane, a place is a phase and its columns: `3` is its first, `3.2` its second and `3.2-4` the second to the
fourth. A box stacks below whatever in its lane already stands over its columns. A line of a box is small words, with
`**bold**` and `` `code` `` as in text; a line of capitals is a heading in its lane's colour. A mark written before a
line's words leads it, and one after sits at the box's right edge.

In an assembly, cells are a column letter and a row number, `B2`, or a span from the top left cell to the bottom right,
`C3-C5`. Parts may not share a cell, and a group holds at least one part whole and does not meet another group. A
balloon is written in a part's lines. `[words]` is a chip, a small box such as a gate; in a store, a line of chips alone
is a row of compartments, each chip's balloon inside it. `📄` draws a folded page.

## Text and terminals

In text, `**bold**`, `` `code` ``, `@actor{words}` for words in an actor's colour, `{n}` for part n's balloon, `▲n` for
note n's flag, `◆n` for item ◆n's diamond and `●n` for item ●n's dot. In a terminal, a line starting `$` is a command
and a first word indented at most three spaces is a key; `{i:id}`, `{h:hash}`, `{ok:done}`, `{q:question}` and
`{a:option}` mark the rest. A backslash keeps the next character as it is.

## Themes

A theme is a stylesheet, such as a project's design tokens. The page inlines it, and each `url(...)` in it, read from
beside the stylesheet, so nothing is fetched. It must define `--canvas`, `--surface`, `--surface-raised`,
`--surface-sunken`, `--line`, `--line-strong`, `--ink`, `--ink-2`, `--font-sans` and `--font-mono`, or it is refused
with those it lacks. The sheet is drawn on canvas, its cells on surface, its heavy edges in line-strong and its
hairlines in line; every face is the theme's sans but code and the wordmark, which are its mono. There is one look,
kept in print: P does nothing, and the hover highlight is ink. Text is measured as Atkinson Hyperlegible Next sets it,
its capitals a fifth wider than the paper's condensed face, so a drawing that fits on paper may need room under a theme.

With a theme, an actor's colour is the name of one of its colour tokens, such as `you`, `agent` or `ink-2`, and an
actor may share that name. A box, chip, rung or typed command in an actor's colour is filled with `--<colour>-tint`
when the theme has one, and otherwise is a raised surface or bare, its edge and headings in the colour; a lane is a
faint band of the tint or bare, and keeps the colour's rule at its left. A terminal takes
`--syn-name` for a key, `--ref` for a hash, `--ok`, `--you-ink` for a question and `--agent` for an option, each ink
when the theme has none. The dot is for a theme whose diamond belongs to someone else, such as a circle that is yours.

A themed sheet is read on a dark screen, so it is set a size larger, in two weights, its boxes and lines spaced to
match, and its dotted lines heavier. Its symbols keep the legend's meanings: a circle is a part's balloon, so phase and
view numbers are plain numerals; dashed is planned, so a detail's frame is a solid line. A flag is larger, an outline
in its actor's colour, or ink-2, lit solid. An actor coloured `ink-2` keeps that grey for its legend swatch and its
lane's rule, and its boxes take ink headings and strong edges, so they do not read as disabled.
