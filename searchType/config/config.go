package config

import (
	"errors"
	"flag"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	defaultTimeout          = 3 * time.Second
	defaultLogPeriod        = 3 * time.Second
	defaultRetry            = 0
	defaultRps              = 1
	defaultCsvSeparator     = "\t"
	defaultPresetsSeparator = ","
)

func parseEnvironment() (int, error) {
	limit := os.Getenv("GO_ERR_GROUP_LIMIT")

	if limit == "" {
		limit = "100"
	}

	var (
		err   error
		value int
	)

	if value, err = strconv.Atoi(limit); err != nil || value <= 0 {
		return 0, ErrInvalidateGroupLimit
	}

	return value, nil
}

type Config struct {
	Rps               int
	QueriesPath       string
	PresetsSeparator  string
	CsvSeparator      string
	ExtendMatchURL    *url.URL
	CountOfRetry      int
	Timeout           time.Duration
	LogPeriod         time.Duration
	GoErrGroupLimiter int
}

func Parse() (*Config, error) {
	cfg := Config{}

	var ExtendMatchURLStr string

	flag.StringVar(&cfg.QueriesPath, "queries-path", "", "Path to list of queries")
	flag.StringVar(&cfg.PresetsSeparator, "presets-separator", defaultPresetsSeparator, "Separator of presets(writing)")
	flag.StringVar(
		&cfg.CsvSeparator,
		"csv-separator",
		defaultCsvSeparator,
		"Separator of csv file with text of search and query part of url(writing)",
	)
	flag.StringVar(&ExtendMatchURLStr, "extend-match-url", "", "Url of extend match machine")
	flag.IntVar(&cfg.Rps, "rps", defaultRps, "Request per second")
	flag.IntVar(&cfg.CountOfRetry, "retry", defaultRetry, "Count of retry per request to extend search server")
	flag.DurationVar(&cfg.Timeout, "timeout", defaultTimeout, "Timeout of making request to server(duration format)")
	flag.DurationVar(&cfg.LogPeriod, "logger-period", defaultLogPeriod, "Period of logger's printing(duration format)")
	flag.Parse()

	if err := cfg.validate(ExtendMatchURLStr); err != nil {
		return nil, err
	}

	var err error

	cfg.GoErrGroupLimiter, err = parseEnvironment()

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (*Config) checkURL(urlStr string) (*url.URL, error) {
	u, err := url.Parse(urlStr)

	if err != nil {
		return nil, err
	}

	if u.Scheme == "" {
		return u, ErrSchema
	}

	if u.Host == "" {
		return u, ErrHost
	}

	return u, nil
}

func (c *Config) validate(urlStr string) error {
	var (
		err, errSum error
		u           *url.URL
	)

	if _, err = os.Stat(c.QueriesPath); err != nil {
		errSum = errors.Join(errSum, err)
	}

	if u, err = c.checkURL(urlStr); err != nil {
		errSum = errors.Join(errSum, err)
	}

	c.ExtendMatchURL = u

	if c.Rps <= 0 {
		errSum = errors.Join(errSum, ErrRps)
	}

	if c.Timeout <= 0 {
		errSum = errors.Join(errSum, ErrTimeout)
	}

	if c.CountOfRetry < 0 {
		errSum = errors.Join(errSum, ErrRetry)
	}

	if c.LogPeriod <= 0 {
		errSum = errors.Join(errSum, ErrLoggerPeriod)
	}

	return errSum
}
