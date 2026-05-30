package ptz

import "errors"

var (
	// ErrPresetNotFound is returned when a preset token does not exist.
	ErrPresetNotFound = errors.New("preset not found")
)
