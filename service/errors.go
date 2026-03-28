package service

import "errors"

// ErrNotFound indicates the requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidInput indicates the caller provided invalid parameters.
var ErrInvalidInput = errors.New("invalid input")
