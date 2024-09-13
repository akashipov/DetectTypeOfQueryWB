package executor

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyResponse  = errors.New("empty response")
	ErrResponseStatus = errors.New("bad status of response")
	ErrNoOnBucket     = errors.New("there is no preset on bucket")
)

func ErrBadResponseStatus(status int, body string) error {
	return fmt.Errorf(
		"please check parameters of request, status - %d, answer - %q: %w", status, body, ErrResponseStatus,
	)
}
