package executor

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/akashipov/DetectTypeOfQueryWB/config"
	errorCustom "github.com/akashipov/DetectTypeOfQueryWB/error"
	"github.com/akashipov/DetectTypeOfQueryWB/logger"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/errgroup"
)

type QueryExecutor struct {
	queries        chan string
	ResponseBodies chan []byte
	rateLimiter    *time.Ticker
	done           chan struct{}
	client         *resty.Client
	cfg            *config.Config
}

func NewQueryExecutor(cfg *config.Config, queries chan string, done chan struct{}) *QueryExecutor {
	ticker := time.NewTicker(time.Second / time.Duration(cfg.Rps))
	return &QueryExecutor{
		queries:        queries,
		ResponseBodies: make(chan []byte),
		cfg:            cfg,
		rateLimiter:    ticker,
		done:           done,
		client:         resty.New().SetTimeout(time.Second * time.Duration(cfg.TimeoutInSec)).SetRetryCount(cfg.CountOfRetry),
	}
}

func (w *QueryExecutor) Run() (err error) {
	defer func() {
		w.rateLimiter.Stop()
		close(w.ResponseBodies)
		logger.GlobalLogger.Add(logger.Info, "Worker has finished")
	}()
	q := w.cfg.ExtendMatchUrl.Query()
	errGroup := errgroup.Group{}
	defer func() {
		err = errGroup.Wait()
	}()
loopTasks:
	for query := range w.queries {
	loopLimit:
		for {
			select {
			case <-w.rateLimiter.C:
				break loopLimit
			case <-w.done:
				continue loopTasks
			}
		}
		q.Set("query", query)
		w.cfg.ExtendMatchUrl.RawQuery = q.Encode()
		errGroup.Go(func() error {
			return w.do(w.cfg.ExtendMatchUrl.String())
		})
	}
	return nil
}

func (w *QueryExecutor) checkResponse(resp *resty.Response) ([]byte, error) {
	body := resp.Body()
	if resp.StatusCode()/100 != 2 {
		return nil, errorCustom.BadResponseError(resp.StatusCode(), string(body))
	}
	rErr := errorCustom.ResponseError{}
	err := json.Unmarshal(body, &rErr)
	if err != nil {
		return nil, err
	}
	if rErr.Code != 0 {
		return nil, errors.New(rErr.Error)
	}
	return body, nil
}

func (w *QueryExecutor) do(urlRequest string) error {
	response, err := w.client.R().Get(urlRequest)
	if err != nil {
		return err
	}
	body, err := w.checkResponse(response)
	if err != nil {
		return err
	}
	w.ResponseBodies <- body
	return nil
}
