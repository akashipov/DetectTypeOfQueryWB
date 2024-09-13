package config

import (
	"errors"
)

type ErrURL error

var (
	ErrSchema ErrURL = errors.New("please setup schema for url")
	ErrHost   ErrURL = errors.New("please setup host for url")
)

var (
	ErrLoggerPeriod         = errors.New("logger period cannot be less than 1 milli-second")
	ErrTimeout              = errors.New("timeout per request cannot be less than zero or equal")
	ErrRetry                = errors.New("retry per request cannot be less than zero")
	ErrRps                  = errors.New("request per second cannot be less than zero or equal")
	ErrInvalidateGroupLimit = errors.New("validation hasn't been passed for 'GO_ERR_GROUP_LIMIT' env, cannot be less or equal zero")
)
