package reader

import (
	"bufio"
	"os"

	"github.com/akashipov/DetectTypeOfQueryWB/logger"
)

type QueriesReader struct {
	pathToQueries string
	Queries       chan string
	done          chan struct{}
}

func NewQueriesReader(pathToQueries string, done chan struct{}) *QueriesReader {
	return &QueriesReader{
		done:          done,
		Queries:       make(chan string),
		pathToQueries: pathToQueries,
	}
}

func (p *QueriesReader) Run() error {
	defer func() {
		close(p.Queries)
		logger.GlobalLogger.Add(logger.Info, "QueriesReader has finished")
	}()

	file, err := os.Open(p.pathToQueries)
	if err != nil {
		return err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			logger.GlobalLogger.AddError(err)
		}
	}()

	reader := bufio.NewScanner(file)
	var bytes []byte
loop:
	for reader.Scan() {
		select {
		case <-p.done:
			break loop
		default:
			bytes = reader.Bytes()
			p.Queries <- string(bytes)
		}
	}
	return nil
}
