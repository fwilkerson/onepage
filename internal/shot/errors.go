package shot

import "errors"

var (
	ErrNoChrome = errors.New("no Chrome or Chromium found")
	ErrNoImage  = errors.New("no image was written")
	ErrNoDOM    = errors.New("no DOM was written")
	ErrClip     = errors.New("the clip reaches past the image")
	ErrNotAPNG  = errors.New("not a PNG")

	errNotPNG = errors.New("the browser exited without a whole PNG at the path")
	errNoHTML = errors.New("the browser exited without writing a whole page")
)
