package settings

import (
	"errors"
)

var (
	ErrReadFile     = errors.New("settings: open file error")
	ErrFileJsonOpen = errors.New("settings: parse file error")
	ErrValiation    = errors.New("settings: validation error")
)
