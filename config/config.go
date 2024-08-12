package config

import (
	"errors"
	"flag"
	"net/url"
	"os"

	errorCustom "github.com/akashipov/DetectTypeOfQueryWB/error"
)

type Config struct {
	Rps              int
	QueriesPath      string
	PresetsSeparator string
	CsvSeparator     string
	ExtendMatchUrl   *url.URL
	CountOfRetry     int
	TimeoutInSec     int
	LogPeriodMilli   int
}

func Parse() (*Config, error) {
	cfg := Config{}
	var ExtendMatchUrlStr string
	flag.StringVar(&cfg.QueriesPath, "queries-path", "", "Path to list of queries")
	flag.StringVar(&cfg.PresetsSeparator, "presets-separator", ",", "Separator of presets(writing)")
	flag.StringVar(&cfg.CsvSeparator, "csv-separator", "\t", "Separator of csv file with text of search and query part of url(writing)")
	flag.StringVar(&ExtendMatchUrlStr, "extend-match-url", "", "Url of extend match machine")
	flag.IntVar(&cfg.Rps, "rps", 1, "Request per second")
	flag.IntVar(&cfg.CountOfRetry, "retry", 0, "Count of retry per request to extend search server")
	flag.IntVar(&cfg.TimeoutInSec, "timeout-in-sec", 3, "Timeout of making request to extend server in seconds")
	flag.IntVar(&cfg.LogPeriodMilli, "logger-period-misec", 3000, "Period of logger's printing in milli-seconds")
	flag.Parse()

	err := cfg.validate(ExtendMatchUrlStr)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) checkURL(URL string) (urlReq *url.URL, errSum error) {
	u, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		errSum = errors.Join(errSum, errorCustom.SchemaError)
	}
	if u.Host == "" {
		errSum = errors.Join(errSum, errorCustom.HostError)
	}
	if errSum != nil {
		return nil, errSum
	}
	return u, nil
}

func (c *Config) validate(URL string) (errSum error) {
	var (
		err error
		u   *url.URL
	)
	if _, err = os.Stat(c.QueriesPath); err != nil {
		errSum = errors.Join(errSum, err)
	}
	if u, err = c.checkURL(URL); err != nil {
		errSum = errors.Join(errSum, err)
	}
	c.ExtendMatchUrl = u
	if c.Rps <= 0 {
		errSum = errors.Join(errSum, errorCustom.RpsError)
	}
	if c.TimeoutInSec <= 0 {
		errSum = errors.Join(errSum, errorCustom.TimeoutError)
	}
	if c.CountOfRetry < 0 {
		errSum = errors.Join(errSum, errorCustom.RetryError)
	}
	if c.LogPeriodMilli <= 0 {
		errSum = errors.Join(errSum, errorCustom.LoggerPeriodError)
	}
	return errSum
}
