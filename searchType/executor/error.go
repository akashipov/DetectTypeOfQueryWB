package executor

import (
	"errors"
	"fmt"
)

type ErrResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

var ErrResponseStatus = errors.New("bad status of response")

func ErrBadResponseStatus(status int, body string) error {
	return fmt.Errorf(
		"please check parameters of request, status - %d, answer - %q: %w", status, body, ErrResponseStatus,
	)
}
