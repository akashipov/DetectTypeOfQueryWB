package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wbx-script/queryVisualizer/config"
	"wbx-script/queryVisualizer/customerror"
	"wbx-script/queryVisualizer/screenshots"
	"wbx-script/searchType/logger"

	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/errgroup"
)

const (
	comma = ","
)

const (
	resultsFolderName = "visualizer_results"
	cardsFileSuffix   = "_cards.ids"
)

const (
	badPresetMsg = "preset param malformed"
)

type Executor struct {
	cfg              *config.ExecutorConfig
	client           *resty.Client
	rateLimit        time.Duration
	makerScreenshots *screenshots.ScreenshotMaker
	lines            <-chan []string
}

func NewExecutor(
	cfg *config.ExecutorConfig,
	makerScreenshots *screenshots.ScreenshotMaker,
	lines <-chan []string,
) *Executor {
	client := resty.New().
		SetTimeout(cfg.BucketRequestsTimeout).
		SetRetryCount(cfg.BucketRequestsRetry).
		SetDisableWarn(true)

	return &Executor{
		cfg:              cfg,
		client:           client,
		rateLimit:        time.Duration(cfg.RatePerSecond),
		makerScreenshots: makerScreenshots,
		lines:            lines,
	}
}

func (*Executor) doWeIgnoreTheseMistakes(err error) bool {
	if errors.Is(err, ErrNoOnBucket) || errors.Is(err, ErrEmptyResponse) {
		// just print to not miss info about
		logger.Info(err.Error())
		return true
	}

	return false
}

func (e *Executor) Run(ctx context.Context) (err error) {
	limiter := time.NewTicker(time.Second / e.rateLimit)
	errGroup, errGroupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(e.cfg.GoErrGroupLimiter)

	defer func() {
		err = errors.Join(errGroup.Wait(), err)

		limiter.Stop()
	}()

	for {
		select {
		case line, ok := <-e.lines:
			if !ok {
				return err
			}

			errGroup.Go(
				func() error {
					errProcess := e.processQuery(
						errGroupCtx, line[0], line[1], limiter,
					)

					if !e.doWeIgnoreTheseMistakes(errProcess) {
						return errProcess
					}

					return nil
				},
			)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (*Executor) waitLimiterAllowed(ctx context.Context, limiter *time.Ticker) error {
	select {
	case <-limiter.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Executor) processQuery(ctx context.Context, text, query string, limiter *time.Ticker) (err error) {
	defer func() {
		if err != nil {
			err = customerror.ErrWrapStack(fmt.Sprintf("processQuery for %q", text), err)
		}

		logger.Info(fmt.Sprintf("%q query has finished", text))
	}()

	var queryParams url.Values

	queryParams, err = url.ParseQuery(query)

	if err != nil {
		return err
	}

	if errLimiter := e.waitLimiterAllowed(ctx, limiter); errLimiter != nil {
		return errLimiter
	}

	var response *resty.Response

	response, err = e.client.R().SetContext(ctx).Get(
		e.setupURL(queryParams).String(),
	)

	if err != nil {
		return err
	}

	if !response.IsSuccess() {
		if response.StatusCode() == http.StatusBadRequest && response.String() == badPresetMsg {
			return fmt.Errorf("preset id - %q, error msg - %q: %w", queryParams["preset"], badPresetMsg, ErrNoOnBucket)
		}

		return ErrBadResponseStatus(response.StatusCode(), response.String())
	}

	return e.process(text, response.Body())
}

func buildPresetsString(products []Product) (string, error) {
	var err error

	builder := strings.Builder{}

	nmID := strconv.Itoa(int(products[0].ID))

	if _, err = builder.WriteString(nmID); err != nil {
		return "", err
	}

	for _, elem := range products[1:] {
		if _, err = builder.WriteString(comma); err != nil {
			return "", err
		}

		nmID = strconv.Itoa(int(elem.ID))

		if _, err = builder.WriteString(nmID); err != nil {
			return "", err
		}
	}

	return builder.String(), nil
}

func (*Executor) writeCards(writer *bufio.Writer, responseProducts []Product) (string, error) {
	cardsIds, err := buildPresetsString(responseProducts)

	if err != nil {
		return "", err
	}

	if _, err = writer.WriteString(cardsIds); err != nil {
		return "", err
	}

	if err1 := writer.Flush(); err1 != nil {
		return "", err1
	}

	return cardsIds, err
}

func (e *Executor) getPrefixFilePath(text string) (string, error) {
	directory := filepath.Join(e.cfg.ResultsPath, resultsFolderName, text)

	if err := os.MkdirAll(directory, os.ModePerm); err != nil {
		return "", err
	}

	prefixFilePath := filepath.Join(directory, e.cfg.ResultsVersionName)

	return prefixFilePath, nil
}

func (*Executor) validateData(body []byte, resp *Response) error {
	if err := json.Unmarshal(body, resp); err != nil {
		return err
	}

	if len(resp.Data.Products) == 0 {
		return ErrEmptyResponse
	}

	return nil
}

func (e *Executor) writePresets(prefixFilePath string, body []byte) (cardsIds string, err error) {
	response := Response{}

	if errValidation := e.validateData(body, &response); errValidation != nil {
		return "", errValidation
	}

	var file *os.File

	file, err = os.OpenFile(prefixFilePath+cardsFileSuffix, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)

	if err != nil {
		return "", err
	}

	defer func() {
		err = errors.Join(err, file.Close())
	}()

	cardsIds, err = e.writeCards(bufio.NewWriter(file), response.Data.Products)

	if err != nil {
		return "", err
	}

	return cardsIds, nil
}

func (e *Executor) process(text string, body []byte) (err error) {
	var prefixFilePath, cardsIds string
	prefixFilePath, err = e.getPrefixFilePath(text)

	if err != nil {
		return
	}

	cardsIds, err = e.writePresets(prefixFilePath, body)

	if err != nil {
		return
	}

	return e.makerScreenshots.MakeScreenshot(cardsIds, prefixFilePath)
}

func (e *Executor) setupURL(queryParams url.Values) *url.URL {
	bucketURL := *e.cfg.BucketServerURL
	q := bucketURL.Query()

	for param, vals := range queryParams {
		if len(vals) == 0 {
			continue
		}

		q.Set(param, vals[0])

		for _, val := range vals[1:] {
			q.Add(param, val)
		}
	}

	bucketURL.RawQuery = q.Encode()

	return &bucketURL
}
