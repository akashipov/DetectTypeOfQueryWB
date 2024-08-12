package error

import "fmt"

type UrlError error

var HostError UrlError = fmt.Errorf("please setup host for url")
var SchemaError UrlError = fmt.Errorf("please setup schema for url")

var RpsError = fmt.Errorf("request per second cannot be less than zero or equal")
var RetryError = fmt.Errorf("retry per request cannot be less than zero")
var TimeoutError = fmt.Errorf("timeout per request cannot be less than zero or equal")
var LoggerPeriodError = fmt.Errorf("logger period cannot be less than 1 milli-second")
