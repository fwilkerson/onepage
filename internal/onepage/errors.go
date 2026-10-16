package onepage

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownBlock   = errors.New("unknown block")
	ErrUnknownKeyword = errors.New("unknown keyword")
	ErrUnknownActor   = errors.New("unknown actor")
	ErrUnknownColour  = errors.New("unknown colour")
	ErrReserved       = errors.New("reserved by the theme")
	ErrUnknownMark    = errors.New("unknown mark")
	ErrUnknownBox     = errors.New("unknown box")
	ErrDuplicate      = errors.New("named twice")
	ErrOverlap        = errors.New("overlaps")
	ErrUnknownLane    = errors.New("unknown lane")
	ErrUnknownPhase   = errors.New("unknown phase")
	ErrUnknownDetail  = errors.New("unknown detail")
	ErrTooWide        = errors.New("wider than the sheet")
	ErrNoWay          = errors.New("no way for the arrow")
	ErrHeadOnTurn     = errors.New("an arrow's head stands on its turn")
	ErrUnknownPart    = errors.New("unknown part")
	ErrUnknownNote    = errors.New("unknown note")
	ErrUnknownDiamond = errors.New("unknown diamond")
	ErrUnknownDot     = errors.New("unknown dot")
	ErrNoTheme        = errors.New("no theme")
	ErrMissingToken   = errors.New("missing token")
	ErrUnkeyedMore    = errors.New("a detail with no mark more in the legend")
	ErrFields         = errors.New("wrong number of fields")
	ErrUnclosed       = errors.New("never closed")
)

type SourceError struct {
	Path string
	Line int
	Err  error
}

func (e *SourceError) Error() string { return fmt.Sprintf("%s:%d: %v", e.Path, e.Line, e.Err) }

func (e *SourceError) Unwrap() error { return e.Err }
