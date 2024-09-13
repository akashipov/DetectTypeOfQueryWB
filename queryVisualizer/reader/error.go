package reader

import (
	"errors"
	"fmt"
)

var ErrColumnsAmount = errors.New("wrong number of columns")

func ErrBadNumberOfColumns(numberOfColumns int) error {
	return fmt.Errorf("it doesn't equal 2(text, query), have got '%d': %w", numberOfColumns, ErrColumnsAmount)
}
