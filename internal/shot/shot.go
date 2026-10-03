package shot

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var Browsers = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"google-chrome",
	"google-chrome-stable",
	"chromium",
	"chromium-browser",
	"chrome",
}

func Find(names []string) (string, error) {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: looked for %s; install Google Chrome or Chromium, or name one with -chrome <path>",
		ErrNoChrome, strings.Join(names, ", "))
}

func Target(arg string) (string, error) {
	if u, err := url.Parse(arg); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "file") {
		return arg, nil
	}
	file, fragment, _ := strings.Cut(arg, "#")
	abs, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs), Fragment: fragment}).String(), nil
}

func Render(ctx context.Context, chrome, target, out string, width, height int, timeout time.Duration) error {
	if err := os.Remove(out); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	runErr, stderr, err := run(ctx, chrome, target, width, height, timeout, nil, func() bool { return whole(out) }, "--screenshot="+out)
	if err != nil {
		return err
	}
	if whole(out) {
		return nil
	}
	err = fmt.Errorf("%w at %s from %s: %w\n%s", ErrNoImage, out, target, cmp.Or(runErr, errNotPNG), stderr)
	if rmErr := os.Remove(out); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return errors.Join(err, rmErr)
	}
	return err
}

func DOM(ctx context.Context, chrome, target string, width, height int, timeout time.Duration) ([]byte, error) {
	var out lockedBuffer
	wholeDOM := func() bool { return bytes.Contains(out.Bytes(), []byte("</html>")) }
	runErr, stderr, err := run(ctx, chrome, target, width, height, timeout, &out, wholeDOM, "--dump-dom")
	if err != nil {
		return nil, err
	}
	if !wholeDOM() {
		return nil, fmt.Errorf("%w from %s: %w\n%s", ErrNoDOM, target, cmp.Or(runErr, errNoHTML), stderr)
	}
	return out.Bytes(), nil
}

func run(ctx context.Context, chrome, target string, width, height int, timeout time.Duration,
	stdout io.Writer, done func() bool, output ...string,
) (runErr error, stderr string, err error) {
	profile, err := os.MkdirTemp("", "shot-profile-")
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = os.RemoveAll(profile) }()

	ctx, cancel := context.WithTimeoutCause(ctx, timeout, fmt.Errorf("killed after %s", timeout))
	defer cancel()
	args := []string{
		"--force-prefers-reduced-motion",
		"--headless=new",
		"--hide-scrollbars",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--use-mock-keychain",
		"--password-store=basic",
		"--force-device-scale-factor=1",
		"--user-data-dir=" + profile,
		fmt.Sprintf("--window-size=%d,%d", width, height),
	}
	cmd := exec.CommandContext(ctx, chrome, slices.Concat(args, output, []string{target})...)
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = stdout, &log
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start %s: %w", chrome, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case runErr = <-exited:
			if ctx.Err() != nil {
				runErr = context.Cause(ctx)
			}
			return runErr, tail(log.String(), 20), nil
		case <-tick.C:
			if done() {
				cancel()
				<-exited
				return nil, "", nil
			}
		}
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) Bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Clone(l.b.Bytes())
}

func Clip(path string, r image.Rectangle) error {
	img, err := decode(path)
	if err != nil {
		return err
	}
	if !r.In(img.Bounds()) || r.Empty() {
		return fmt.Errorf("%w: %v in an image of %v", ErrClip, r, img.Bounds())
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return fmt.Errorf("clip %s: a %T cannot be cropped", path, img)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, sub.SubImage(r)); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func whole(path string) bool {
	img, err := os.ReadFile(path)
	return err == nil && bytes.HasPrefix(img, []byte("\x89PNG\r\n\x1a\n")) &&
		bytes.HasSuffix(img, []byte("IEND\xaeB`\x82"))
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

func decode(path string) (image.Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", path, ErrNotAPNG, err)
	}
	return img, nil
}
