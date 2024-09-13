package config

import (
	"errors"
	"fmt"
)

var ErrInvalidParameter = errors.New("invalid parameter")

func ErrWrapInvalidParameter(name string, err error) error {
	return fmt.Errorf("%q parameter error: %w", name, errors.Join(err, ErrInvalidParameter))
}

var ErrEmptyURL = errors.New("empty schema or host of URL")
