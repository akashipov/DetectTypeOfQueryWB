package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"wbx-script/searchType/config"
	"wbx-script/searchType/executor"
	"wbx-script/searchType/logger"
	"wbx-script/searchType/reader"
	"wbx-script/searchType/saver"

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

	logger.GlobalLogger = logger.NewLogger(
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		cfg.LogPeriod,
	)

	logger.GlobalLogger.Run()

	defer logger.GlobalLogger.Stop()

	queriesReader := reader.NewQueriesReader(cfg.QueriesPath)
	queryExecutor := executor.NewQueryExecutor(cfg, queriesReader.Queries)
	writer, err := saver.NewSaver(cfg, queryExecutor.ResponseBodies)

	if err != nil {
		logger.Error(err.Error())
		return
	}

	signalCtx, signalStop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	defer signalStop()

	app := newChain(queriesReader.Run, queryExecutor.Run, writer.Run)

	app.run(signalCtx, cfg.GoErrGroupLimiter)
}
