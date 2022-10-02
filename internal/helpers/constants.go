package helpers

import (
	"os"
	"strconv"
)

const (
	DEFAULT_TIMEOUT = 1
)

func GetPutTimeout() int {
	if value, ok := os.LookupEnv("SQUARES_PUT_TIMEOUT"); ok {
		timeout, _ := strconv.Atoi(value)
		return timeout
	}
	return DEFAULT_TIMEOUT
}
