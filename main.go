package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/fwilkerson/onepage/internal/onepage"
	"github.com/fwilkerson/onepage/internal/shot"
)

//go:embed grammar.md
var grammar string

const usage = `usage: onepage <command> [arguments]

  render <source> <page>   render a one-page design's text source into its page
  shot [flags] <file|url>  render a page to a PNG with headless Chrome; -h lists its flags
  grammar                  print the markup a source is written in
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
	default:
		_, _ = fmt.Fprintln(os.Stderr, "onepage:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return errUsage
	}
	switch args[0] {
	case "render":
		return render(args[1:], stderr)
	case "shot":
		return shoot(ctx, args[1:], stdout, stderr)
	case "grammar":
		_, err := io.WriteString(stdout, grammar)
		return err
	case "help", "-h", "-help", "--help":
		_, err := io.WriteString(stdout, usage)
		return err
	}
	_, _ = io.WriteString(stderr, usage)
	return fmt.Errorf("%w: %q", errUnknownCommand, args[0])
}

func render(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("render <source> <page>", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return fmt.Errorf("render: %w: want a source and a page, such as docs/one-page.txt docs/one-page.html", errArgs)
	}
	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	page, err := onepage.Render(fs.Arg(0), src)
	if err != nil {
		return err
	}
	return os.WriteFile(fs.Arg(1), page, 0o644)
}

func shoot(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("shot [flags] <file|url>", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "shot.png", "where to write the PNG")
	width := fs.Int("width", 1280, "window width in pixels")
	height := fs.Int("height", 800, "window height in pixels; the image shows no more of the page than this")
	chrome := fs.String("chrome", "", "the Chrome or Chromium to run, instead of looking in the usual places")
	timeout := fs.Duration("timeout", 45*time.Second, "how long Chrome may take before it is killed")
	clip := fs.String("clip", "", "x,y,w,h: crop the image to this rectangle of the window")
	blueprint := fs.Bool("blueprint", false, "open the page on the dark blueprint, through its #blueprint hash")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var r image.Rectangle
	if *clip != "" {
		var x, y, w, h int
		if n, err := fmt.Sscanf(*clip, "%d,%d,%d,%d", &x, &y, &w, &h); err != nil || n != 4 {
			return fmt.Errorf("shot: %w, not %q", errClip, *clip)
		}
		r = image.Rect(x, y, x+w, y+h)
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("shot: %w: want a file or a URL, such as docs/one-page.html", errArgs)
	}
	target, err := shot.Target(fs.Arg(0))
	if err != nil {
		return err
	}
	if *blueprint {
		u, err := url.Parse(target)
		if err != nil {
			return err
		}
		u.Fragment = "blueprint"
		target = u.String()
	}
	browsers := shot.Browsers
	if *chrome != "" {
		browsers = []string{*chrome}
	}
	path, err := shot.Find(browsers)
	if err != nil {
		return err
	}
	if err := shot.Render(ctx, path, target, *out, *width, *height, *timeout); err != nil {
		return err
	}
	if *clip != "" {
		if err := shot.Clip(*out, r); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(stdout, *out)
	return err
}
