package store

import "errors"

// ErrConflict is returned when an object_id already exists.
var ErrConflict = errors.New("object already exists")

// ErrNotFound is returned when an object is not found.
var ErrNotFound = errors.New("object not found")
