package customerror

import "fmt"

func ErrWrapStack(functionName string, err error) error {
	return fmt.Errorf("%s -> %w", functionName, err)
}
