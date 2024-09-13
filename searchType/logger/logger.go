package logger

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Level string

const (
	InfoLevel  Level = "Info"
	ErrorLevel Level = "Error"
)

type logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type log struct {
	level Level
	msg   string
	args  []any
}

type Logger struct {
	logger
	done   chan struct{}
	buffer []log
	ticker *time.Ticker
	mu     *sync.Mutex
	wg     *sync.WaitGroup
}

func NewLogger(logger logger, periodOfLogging time.Duration) *Logger {
	l := Logger{
		done:   make(chan struct{}),
		logger: logger,
		buffer: make([]log, 0),
		ticker: time.NewTicker(periodOfLogging),
		mu:     &sync.Mutex{},
		wg:     &sync.WaitGroup{},
	}

	return &l
}

func (l *Logger) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, v := range l.buffer {
		switch v.level {
		case InfoLevel:
			l.logger.Info(v.msg, v.args...)
		case ErrorLevel:
			l.logger.Error(v.msg, v.args...)
		}
	}

	l.buffer = l.buffer[:0]
}

func (l *Logger) Run() {
	l.wg.Add(1)

	go func() {
		defer func() {
			l.flush()
			fmt.Println("Logger has finished")
			l.wg.Done()
		}()

		for {
			select {
			case <-l.ticker.C:
				l.flush()
			case <-l.done:
				return
			}
		}
	}()
}

func (l *Logger) Stop() {
	l.ticker.Stop()
	close(l.done)
	l.wg.Wait()
}

func (l *Logger) Info(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buffer = append(l.buffer, log{level: InfoLevel, msg: msg, args: args})
}

func (l *Logger) Error(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buffer = append(l.buffer, log{level: ErrorLevel, msg: msg, args: args})
}

func Error(msg string, args ...any) {
	GlobalLogger.Error(msg, args...)
}

func Info(msg string, args ...any) {
	GlobalLogger.Info(msg, args...)
}

var GlobalLogger = NewLogger(
	slog.New(slog.NewTextHandler(os.Stdout, nil)),
	3*time.Second,
)
