package executor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"wbx-script/searchType/config"
	"wbx-script/searchType/logger"

	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/errgroup"
)

type QueryExecutor struct {
	queries        <-chan string
	ResponseBodies chan []byte
	rateLimiter    *time.Ticker
	client         *resty.Client
	cfg            *config.Config
}

func NewQueryExecutor(cfg *config.Config, queries <-chan string) *QueryExecutor {
	ticker := time.NewTicker(time.Second / time.Duration(cfg.Rps))

	return &QueryExecutor{
		queries:        queries,
		ResponseBodies: make(chan []byte),
		cfg:            cfg,
		rateLimiter:    ticker,
		client:         resty.New().SetTimeout(cfg.Timeout).SetRetryCount(cfg.CountOfRetry).SetDisableWarn(true),
	}
}

func (w *QueryExecutor) waitLimiterAllowed(ctx context.Context) error {
	select {
	case <-w.rateLimiter.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *QueryExecutor) Run(ctx context.Context) (err error) {
	defer func() {
		w.rateLimiter.Stop()
		close(w.ResponseBodies)
		logger.Info("Worker has finished")
	}()

	errGroup, errGroupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(w.cfg.GoErrGroupLimiter)

	defer func() {
		err = errors.Join(errGroup.Wait(), err)
	}()

	q := w.cfg.ExtendMatchURL.Query()

	for query := range w.queries {
		if errLimiter := w.waitLimiterAllowed(errGroupCtx); errLimiter != nil {
			return errLimiter
		}

		q.Set("query", query)
		w.cfg.ExtendMatchURL.RawQuery = q.Encode()

		errGroup.Go(func() error {
			return w.do(errGroupCtx, w.cfg.ExtendMatchURL.String())
		})
	}

	return
}

func (*QueryExecutor) checkResponse(resp *resty.Response) ([]byte, error) {
	body := resp.Body()
	status := resp.StatusCode()

	if !resp.IsSuccess() {
		return nil, ErrBadResponseStatus(status, resp.String())
	}

	rErr := ErrResponse{}

	if err := json.Unmarshal(body, &rErr); err != nil {
		return nil, err
	}

	if rErr.Code != 0 {
		return nil, ErrBadResponseStatus(rErr.Code, rErr.Error)
	}

	return body, nil
}

func (w *QueryExecutor) do(ctx context.Context, urlRequest string) error {
	response, err := w.client.R().SetContext(ctx).Get(urlRequest)

	if err != nil {
		return err
	}

	body, err := w.checkResponse(response)

	if err != nil {
		return err
	}

	select {
	case w.ResponseBodies <- body:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
