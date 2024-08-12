package signalprocessing

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/akashipov/DetectTypeOfQueryWB/logger"
)

type SignalProcessing struct {
	Done chan struct{}
	wg   *sync.WaitGroup
}

func NewSignalProcessing() *SignalProcessing {
	s := SignalProcessing{
		make(chan struct{}), &sync.WaitGroup{},
	}
	s.wg.Add(1)
	go s.run()
	return &s
}

func (s *SignalProcessing) run() {
	defer func() {
		s.wg.Done()
		logger.GlobalLogger.Add(logger.Info, "Signal processing has stopped")
	}()
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
loop:
	for {
		select {
		case sig := <-sigint:
			logger.GlobalLogger.Add(logger.Info, fmt.Sprintf("Signal: %v", sig))
			close(s.Done)
			break loop
		case <-s.Done:
			break loop
		}
	}
}

func (s *SignalProcessing) Stop() {
	select {
	case <-s.Done:
		break
	default:
		close(s.Done)
	}
	s.wg.Wait()
}
