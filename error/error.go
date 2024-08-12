package error

import (
	"fmt"
)

type ResponseError struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func BadResponseError(status int, text string) error {
	return fmt.Errorf("bad response status - %d, msg - %s", status, text)
}

var InvalidEmptyResponse = fmt.Errorf("invalid emtpy response")

func InvalidResponseSeparator(text, sep string) error {
	return fmt.Errorf("response - '%s' - contains csv separator '%s'", text, sep)
}
