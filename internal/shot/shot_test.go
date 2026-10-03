package shot_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fwilkerson/onepage/internal/shot"
)

func TestFindNamesEveryPlaceItSearched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := []string{filepath.Join(dir, "Google Chrome"), "no-such-chrome-binary"}
	_, err := shot.Find(missing)
	if !errors.Is(err, shot.ErrNoChrome) {
		t.Fatalf("err = %v, want ErrNoChrome", err)
	}
	for _, name := range append(missing, "-chrome") {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("err = %q, want it to name %q", err, name)
		}
	}

	chrome := filepath.Join(dir, "chrome")
	if err := os.WriteFile(chrome, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := shot.Find(append(missing, chrome))
	if err != nil || got != chrome {
		t.Errorf("Find = %q, %v, want %q", got, err, chrome)
	}
}

func TestTargetLoadsAURLAsGivenAndAFileOnlyWhenItExists(t *testing.T) {
	t.Parallel()
	for _, u := range []string{"http://localhost:8080/tasks", "https://example.com", "file:///tmp/x.html"} {
		if got, err := shot.Target(u); err != nil || got != u {
			t.Errorf("Target(%q) = %q, %v, want it as given", u, got, err)
		}
	}

	dir := t.TempDir()
	missing := filepath.Join(dir, "gone.html")
	if _, err := shot.Target(missing); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), missing) {
		t.Errorf("Target(missing) err = %v, want it to name %s as not existing", err, missing)
	}

	page := filepath.Join(dir, "a page.html")
	if err := os.WriteFile(page, []byte("<p>hi</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := shot.Target(page)
	if err != nil || got != "file://"+strings.ReplaceAll(page, " ", "%20") {
		t.Errorf("Target(page) = %q, %v, want its file URL", got, err)
	}
}

func TestRenderWritesAPNGOfTheAskedSize(t *testing.T) {
	t.Parallel()
	chrome, err := shot.Find(shot.Browsers)
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte(`<body style="background:#c00">shot</body>`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "img", "shot.png")
	url, err := shot.Target(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := shot.Render(t.Context(), chrome, url, out, 640, 360, time.Minute); err != nil {
		t.Fatal(err)
	}
	img, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(img, []byte("\x89PNG\r\n\x1a\n")) || len(img) < 24 {
		t.Fatalf("out is %.16q, want a PNG", img)
	}
	w, h := binary.BigEndian.Uint32(img[16:20]), binary.BigEndian.Uint32(img[20:24])
	if w != 640 || h != 360 {
		t.Errorf("the image is %dx%d, want 640x360", w, h)
	}
}

func TestRenderShowsThePageSettled(t *testing.T) {
	t.Parallel()
	chrome, err := shot.Find(shot.Browsers)
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	src := `<style>
body { margin: 0; background: #00c; animation: in 60s; }
@keyframes in { from { background: #c00; } to { background: #c00; } }
@media (prefers-reduced-motion: reduce) { body { animation: none; } }
</style><body></body>`
	if err := os.WriteFile(page, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	url, err := shot.Target(page)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "shot.png")
	if err := shot.Render(t.Context(), chrome, url, out, 320, 200, time.Minute); err != nil {
		t.Fatal(err)
	}
	blue := color.RGBA{0, 0, 0xcc, 255}
	if got := color.RGBAModel.Convert(readPNG(t, out).At(10, 10)); got != blue {
		t.Errorf("the shot is %v, want %v: the page settled, not mid-animation", got, blue)
	}
}

func TestClipCropsAPNGToARectangle(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "shot.png")
	src := image.NewRGBA(image.Rect(0, 0, 40, 20))
	red := color.RGBA{200, 0, 0, 255}
	for x := 20; x < 40; x++ {
		for y := range 20 {
			src.Set(x, y, red)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := shot.Clip(path, image.Rect(25, 5, 35, 15)); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	clipped, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(clipped))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != image.Rect(0, 0, 10, 10) || color.RGBAModel.Convert(got.At(0, 0)) != red {
		t.Errorf("clipped to %v, first pixel %v, want 10x10 and red", got.Bounds(), got.At(0, 0))
	}
	if err := shot.Clip(path, image.Rect(5, 5, 50, 15)); !errors.Is(err, shot.ErrClip) {
		t.Errorf("clip past the edge: err = %v, want ErrClip", err)
	}
}

func writePNG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestDOMReturnsThePageAfterItsScriptsRan(t *testing.T) {
	t.Parallel()
	chrome, err := shot.Find(shot.Browsers)
	if err != nil {
		t.Skip(err)
	}
	page := filepath.Join(t.TempDir(), "page.html")
	src := `<body><p id="w"></p><script>w.textContent = "width " + innerWidth</script></body>`
	if err := os.WriteFile(page, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	url, err := shot.Target(page)
	if err != nil {
		t.Fatal(err)
	}
	dom, err := shot.DOM(t.Context(), chrome, url, 640, 360, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(dom, []byte(`<p id="w">width 640</p>`)) {
		t.Errorf("DOM = %s, want the script's width 640 in it", dom)
	}
}

func TestDOMNamesWhyChromeWroteNone(t *testing.T) {
	t.Parallel()
	chrome := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(chrome, []byte("#!/bin/sh\necho '<html>' ; echo 'no display here' >&2; exit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := shot.DOM(t.Context(), chrome, "file:///x.html", 640, 360, time.Minute)
	if !errors.Is(err, shot.ErrNoDOM) || !strings.Contains(err.Error(), "no display here") {
		t.Errorf("err = %v, want ErrNoDOM with Chrome's stderr", err)
	}
}

func TestRenderLeavesNoStaleImageWhenChromeWritesNone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	chrome := filepath.Join(dir, "chrome")
	if err := os.WriteFile(chrome, []byte("#!/bin/sh\necho 'no display here' >&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(out, []byte("\x89PNG\r\n\x1a\nstale"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := shot.Render(t.Context(), chrome, "file:///x.html", out, 640, 360, time.Minute)
	if !errors.Is(err, shot.ErrNoImage) || !strings.Contains(err.Error(), "no display here") {
		t.Errorf("err = %v, want ErrNoImage with Chrome's stderr", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the stale image survived a failed render: %v", err)
	}
}
