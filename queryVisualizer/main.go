package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"wbx-script/queryVisualizer/config"
	"wbx-script/queryVisualizer/executor"
	"wbx-script/queryVisualizer/reader"
	"wbx-script/queryVisualizer/screenshots"
	"wbx-script/searchType/logger"

	"golang.org/x/sync/errgroup"
)

type chain []func(ctx context.Context) error

func newChain(tasks ...func(ctx context.Context) error) chain {
	return tasks
}

func (c chain) run(ctx context.Context, limit int) {
	errGroup, errGroupCtx := errgroup.WithContext(ctx)
	errGroup.SetLimit(limit)

	for i := range c {
		errGroup.Go(func() error { return c[i](errGroupCtx) })
	}

	if err := errGroup.Wait(); err != nil {
		logger.Error(err.Error())
	}
}

func main() {
	cfg, err := config.Parse()

	if err != nil {
		log.Fatal(err)
	}

	logger.GlobalLogger = logger.NewLogger(slog.New(slog.NewTextHandler(os.Stdout, nil)), cfg.LogPeriod)
	logger.GlobalLogger.Run()

	defer logger.GlobalLogger.Stop()

	screenMaker, err := screenshots.NewScreenshotMaker(
		&cfg.ScreenshotMakerConfig,
	)

	if err != nil {
		logger.Error(err.Error())
		return
	}

	defer func() {
		if err = screenMaker.Stop(); err != nil {
			logger.Error(err.Error())
		}
	}()

	lines := make(chan []string)
	exec := executor.NewExecutor(
		&cfg.ExecutorConfig,
		screenMaker,
		lines,
	)

	signalCtx, signalStop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	defer signalStop()

	app := newChain(
		func(c context.Context) error {
			return reader.Read(c, &cfg.ReaderConfig, lines)
		},
		exec.Run,
	)

	app.run(signalCtx, cfg.GoErrGroupLimiter)
}
