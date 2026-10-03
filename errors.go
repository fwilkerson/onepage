package main

import "errors"

var (
	errUsage          = errors.New("no command given")
	errUnknownCommand = errors.New("unknown command")
	errArgs           = errors.New("wrong arguments")
	errClip           = errors.New("-clip wants x,y,w,h, such as 0,1800,1000,700")
)
