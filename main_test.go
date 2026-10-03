package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderNamesItsOwnCommandInThePage(t *testing.T) {
	src := filepath.Join("internal", "onepage", "testdata", "kiln.txt")
	page := filepath.Join(t.TempDir(), "kiln.html")
	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), []string{"render", src, page}, &stdout, &stderr); err != nil {
		t.Fatalf("render: %v\n%s", err, stderr.String())
	}
	b, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	want := "Rendered from " + src + " by `onepage render`"
	if !strings.Contains(string(b), want) {
		t.Errorf("the page does not say %q", want)
	}
	if strings.Contains(string(b), "just ") {
		t.Errorf("the page names a just recipe, which only another repository has")
	}
}

func TestRenderRefusesAnythingButASourceAndAPage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), []string{"render", "one-page.txt"}, &stdout, &stderr)
	if !errors.Is(err, errArgs) {
		t.Errorf("render with one argument: got %v, want %v", err, errArgs)
	}
}

func TestGrammarPrintsTheMarkup(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(t.Context(), []string{"grammar"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "# The one-page markup\n") {
		t.Errorf("grammar printed %.40q", stdout.String())
	}
}

func TestAnUnknownCommandIsRefusedWithTheUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), []string{"draw"}, &stdout, &stderr)
	if !errors.Is(err, errUnknownCommand) {
		t.Errorf("got %v, want %v", err, errUnknownCommand)
	}
	if !strings.Contains(stderr.String(), "render <source> <page>") {
		t.Errorf("stderr holds no usage:\n%s", stderr.String())
	}
}
