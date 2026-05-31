package retry

import "time"

var defaultDelays = []time.Duration{
	time.Second,
	3 * time.Second,
	5 * time.Second,
}

func Do(operation func() error, shouldRetry func(error) bool) error {
	return DoWithSleeper(operation, shouldRetry, time.Sleep)
}

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
