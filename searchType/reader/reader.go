package reader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"wbx-script/searchType/logger"
)

type QueriesReader struct {
	pathToQueries string
	Queries       chan string
}

func NewQueriesReader(pathToQueries string) *QueriesReader {
	return &QueriesReader{
		Queries:       make(chan string),
		pathToQueries: pathToQueries,
	}
}

func (p *QueriesReader) Run(ctx context.Context) (err error) {
	defer func() {
		close(p.Queries)
		logger.Info("QueriesReader has finished")
	}()

	file, err := os.Open(p.pathToQueries)
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, file.Close())
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p.Queries <- text:
			logger.Info(fmt.Sprintf("Query %q has been read", text))
		}
	}

	return
}
