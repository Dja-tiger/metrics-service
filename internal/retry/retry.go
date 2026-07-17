package retry

import "time"

var defaultDelays = []time.Duration{
	time.Second,
	3 * time.Second,
	5 * time.Second,
}

// Do runs an operation and retries it with default delays while shouldRetry returns true.
func Do(operation func() error, shouldRetry func(error) bool) error {
	return DoWithSleeper(operation, shouldRetry, time.Sleep)
}

// DoWithSleeper runs an operation with retry delays delegated to sleep.
func DoWithSleeper(operation func() error, shouldRetry func(error) bool, sleep func(time.Duration)) error {
	err := operation()
	if err == nil || !shouldRetry(err) {
		return err
	}

	for _, delay := range defaultDelays {
		sleep(delay)

		err = operation()
		if err == nil || !shouldRetry(err) {
			return err
		}
	}

	return err
}
