package config

import (
	"flag"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"
)

const (
	defaultRps               = 1
	defaultRetry             = 0
	defaultTimeout           = time.Second * 3
	defaultWidthScale        = 2000
	defaultHeightScale       = 1500
	defaultPagePoolSize      = 10
	defaultScreenshotTimeout = time.Second * 30
	defaultCsvSeparator      = "\t"
)

type ScreenshotMakerConfig struct {
	Width         int
	Height        int
	Timeout       time.Duration
	VisualizerURL *url.URL
	PoolSize      int
}

type ExecutorConfig struct {
	BucketServerURL       *url.URL
	ResultsVersionName    string
	ResultsPath           string
	RatePerSecond         int
	BucketRequestsRetry   int
	BucketRequestsTimeout time.Duration
	GoErrGroupLimiter     int
}

type ReaderConfig struct {
	PathToQueries string
	CsvSeparator  string
}

type LoggerConfig struct {
	LogPeriod time.Duration
}

type Config struct {
	ScreenshotMakerConfig
	ExecutorConfig
	ReaderConfig
	LoggerConfig
}

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
		return 0, ErrWrapInvalidParameter("GO_ERR_GROUP_LIMIT", err)
	}

	return value, nil
}

func Parse() (*Config, error) {
	cfg := Config{}
	flag.IntVar(&cfg.RatePerSecond, "rps", defaultRps, "Number of requests per second")
	flag.IntVar(&cfg.BucketRequestsRetry, "bucket-retry", defaultRetry, "Number of request's retry to bucket server")
	flag.DurationVar(&cfg.BucketRequestsTimeout, "bucket-timeout", defaultTimeout, "Request's timeout to bucket server")
	flag.StringVar(&cfg.PathToQueries, "queries-file-path", "", "Path to csv of queries with text, query columns")
	flag.StringVar(&cfg.ResultsPath, "result-path", "", "Path for saving result files")
	flag.StringVar(&cfg.ResultsVersionName, "results-version-name", "", "Result is saved to files with this prefix")

	var bucketServerURL, VisualizerURL string

	flag.StringVar(&bucketServerURL, "bucket-server-url", "", "URL to bucket server")
	flag.StringVar(&VisualizerURL, "visualizer-server-url", "", "URL to visualizer for screenshots")
	flag.IntVar(&cfg.Width, "screenshot-width", defaultWidthScale, "Scale of screenshots by width")
	flag.IntVar(&cfg.Height, "screenshot-height", defaultHeightScale, "Scale of screenshots by height")
	flag.IntVar(&cfg.PoolSize, "browser-pool-size", defaultPagePoolSize, "Pool of pages on browser to make screenshots")
	flag.DurationVar(&cfg.Timeout, "screenshot-timeout", defaultScreenshotTimeout, "Timeout to make screenshot on chromium driver")
	flag.DurationVar(&cfg.LogPeriod, "logger-period", defaultTimeout, "Period of logger's printing(duration format)")
	flag.StringVar(
		&cfg.CsvSeparator,
		"csv-separator",
		defaultCsvSeparator,
		"Separator of csv file with text of search and query part of url(writing)",
	)

	flag.Parse()

	if cfg.ResultsPath == "" {
		cfg.ResultsPath = filepath.Dir(cfg.PathToQueries)
	}

	if err := cfg.validate(bucketServerURL, VisualizerURL); err != nil {
		log.Fatal(err)
	}

	var err error

	cfg.GoErrGroupLimiter, err = parseEnvironment()

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func parseURL(urlStr, paramName string) (*url.URL, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, ErrWrapInvalidParameter(paramName, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return nil, ErrEmptyURL
	}

	return u, nil
}

func (c *Config) validatePaths() error {
	if info, err := os.Stat(c.PathToQueries); err != nil || info.IsDir() {
		return ErrWrapInvalidParameter("queries-file-path", err)
	} else if info, err = os.Stat(c.ResultsPath); err != nil || !info.IsDir() {
		return ErrWrapInvalidParameter("result-path", err)
	}

	return nil
}

func (c *Config) validate(bucketURLStr, visualizerURLStr string) error {
	if err := c.validatePaths(); err != nil {
		return err
	}

	var err error

	c.BucketServerURL, err = parseURL(bucketURLStr, "bucket-server-url")
	if err != nil {
		return err
	}

	c.VisualizerURL, err = parseURL(visualizerURLStr, "visualizer-server-url")
	if err != nil {
		return err
	}

	switch {
	case c.RatePerSecond <= 0:
		return ErrWrapInvalidParameter("rps", nil)
	case c.Timeout <= 0:
		return ErrWrapInvalidParameter("screenshot-timeout", nil)
	case c.LogPeriod <= 0:
		return ErrWrapInvalidParameter("logger-period", nil)
	case c.Height <= 0:
		return ErrWrapInvalidParameter("screenshot-height", nil)
	case c.Width <= 0:
		return ErrWrapInvalidParameter("screenshot-width", nil)
	case c.PoolSize <= 0:
		return ErrWrapInvalidParameter("browser-pool-size", nil)
	case c.ResultsVersionName == "":
		return ErrWrapInvalidParameter("results-version-name", nil)
	case c.BucketRequestsRetry < 0:
		return ErrWrapInvalidParameter("bucket-retry", nil)
	case c.BucketRequestsTimeout <= 0:
		return ErrWrapInvalidParameter("bucket-timeout", nil)
	case utf8.RuneCountInString(c.CsvSeparator) != 1:
		return ErrWrapInvalidParameter("csv-separator", nil)
	}

	return nil
}
