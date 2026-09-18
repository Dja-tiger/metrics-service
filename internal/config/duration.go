package config

import (
	"fmt"
	"math"
	"time"
)

// Check seconds before callers multiply by time.Second, which can overflow.
func validateSeconds(name string, seconds int) error {
	if int64(seconds) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("%s exceeds maximum supported duration", name)
	}
	return nil
}
