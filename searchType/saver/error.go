package saver

import (
	"errors"
	"fmt"
)

var (
	ErrSeparator       = errors.New("separator error")
	ErrEmptyResponse   = errors.New("empty response")
	ErrUnknownCategory = errors.New("'unknown' category, need to skip'")
)

func ErrInvalidResponseSeparator(text, sep string) error {
	return fmt.Errorf("response - %q - contains csv separator %q: %w", text, sep, ErrSeparator)
}
