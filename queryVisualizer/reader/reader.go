package reader

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"wbx-script/queryVisualizer/config"
	"wbx-script/queryVisualizer/customerror"
	"wbx-script/searchType/logger"
)

const (
	semicolon = ";"
)

func Read(ctx context.Context, cfg *config.ReaderConfig, lines chan<- []string) (err error) {
	defer func() {
		close(lines)

		if err != nil {
			err = customerror.ErrWrapStack("reader", err)
		}

		logger.Info("Reader has finished")
	}()

	f, err := os.Open(cfg.PathToQueries)
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, f.Close())
	}()

	csvReader := csv.NewReader(f)
	sepRune, _ := utf8.DecodeRuneInString(cfg.CsvSeparator)
	csvReader.Comma = sepRune
	isHeader := true

	var record []string

	for {
		record, err = csvReader.Read()

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}

		if isHeader {
			isHeader = false
			continue
		}

		if len(record) != 2 {
			return ErrBadNumberOfColumns(len(record))
		}

		record[1] = strings.ReplaceAll(record[1], semicolon, url.PathEscape(semicolon))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case lines <- record:
		}
	}
}
