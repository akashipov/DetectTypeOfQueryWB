package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Level string

const (
	Info       Level = "Info"
	Error      Level = "Error"
	HeaderSize       = 20
)

var allLevels = []Level{Info, Error}

type Logger struct {
	mapLevelToMessages map[Level][]string
	mu                 *sync.Mutex
	done               chan struct{}
	wg                 *sync.WaitGroup
	ticker             *time.Ticker
}

func NewLogger(periodOfLogging int) *Logger {
	l := Logger{
		make(map[Level][]string, len(allLevels)),
		&sync.Mutex{},
		nil,
		&sync.WaitGroup{},
		time.NewTicker(time.Millisecond * time.Duration(periodOfLogging)),
	}
	return &l
}

var GlobalLogger *Logger

func (l *Logger) getHeader(level Level) string {
	side := strings.Repeat("-", HeaderSize)
	return side + " " + string(level) + " " + side
}

func (l *Logger) getEndHeader(level Level) string {
	return strings.Repeat("-", (HeaderSize+1)*2+len(level))
}

func (l *Logger) print() {
	l.mu.Lock()
	for level, messages := range l.mapLevelToMessages {
		if len(messages) != 0 {
			fmt.Println(l.getHeader(level))
			for i := range messages {
				fmt.Println(messages[i])
			}
			l.mapLevelToMessages[level] = messages[:0]
			fmt.Println(l.getEndHeader(level))
		}
	}
	l.mu.Unlock()
}

func (l *Logger) run() {
	defer func() {
		l.wg.Done()
	}()
	defer l.ticker.Stop()
loop:
	for {
		select {
		case <-l.ticker.C:
			l.print()
		case <-l.done:
			break loop
		}
	}
}
func (l *Logger) Start() {
	l.done = make(chan struct{})
	l.wg.Add(1)
	go l.run()
}

func (l *Logger) Add(level Level, msg string) {
	if msg != "" {
		l.mu.Lock()
		l.mapLevelToMessages[level] = append(l.mapLevelToMessages[level], msg)
		l.mu.Unlock()
	}
}

func (l *Logger) AddError(err error) {
	l.Add(Error, err.Error())
}

func (l *Logger) Stop() {
	if l.done != nil {
		close(l.done)
	}
	l.print()
	l.wg.Wait()
	fmt.Println("Logger has stopped")
}
