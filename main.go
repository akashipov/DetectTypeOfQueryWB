package main

import (
	"log"

	"github.com/akashipov/DetectTypeOfQueryWB/config"
	"github.com/akashipov/DetectTypeOfQueryWB/executor"
	"github.com/akashipov/DetectTypeOfQueryWB/logger"
	"github.com/akashipov/DetectTypeOfQueryWB/reader"
	"github.com/akashipov/DetectTypeOfQueryWB/saver"
	"github.com/akashipov/DetectTypeOfQueryWB/signal"
	"golang.org/x/sync/errgroup"
)

type Chain struct {
	tasksOrdered []func() error
}

func (c *Chain) run() {
	errGroup := errgroup.Group{}
	for i := range c.tasksOrdered {
		errGroup.Go(c.tasksOrdered[i])
	}
	err := errGroup.Wait()
	if err != nil {
		logger.GlobalLogger.AddError(err)
	}
}

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Fatal(err)
	}
	sw := signalprocessing.NewSignalProcessing()
	defer sw.Stop()
	logger.GlobalLogger = logger.NewLogger(cfg.LogPeriodMilli)
	logger.GlobalLogger.Start()
	defer logger.GlobalLogger.Stop()

	queriesReader := reader.NewQueriesReader(cfg.QueriesPath, sw.Done)
	queryExecutor := executor.NewQueryExecutor(cfg, queriesReader.Queries, sw.Done)
	writer, err := saver.NewSaver(cfg, queryExecutor.ResponseBodies, sw.Done)
	if err != nil {
		logger.GlobalLogger.AddError(err)
		return
	}

	chain := Chain{[]func() error{queriesReader.Run, queryExecutor.Run, writer.Run}}
	chain.run()
}
