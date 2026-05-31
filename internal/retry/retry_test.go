package retry

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestDoWithSleeperRetriesWithExpectedDelays(t *testing.T) {
	attempts := 0
	var delays []time.Duration

	err := DoWithSleeper(
		func() error {
			attempts++
			if attempts < 4 {
				return errors.New("temporary error")
			}
			return nil
		},
		func(error) bool {
			return true
		},
		func(delay time.Duration) {
			delays = append(delays, delay)
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 4 {
		t.Fatalf("unexpected attempts count: got %d want 4", attempts)
	}

	wantDelays := []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}
	if !reflect.DeepEqual(delays, wantDelays) {
		t.Fatalf("unexpected delays: got %v want %v", delays, wantDelays)
	}
}

func TestDoWithSleeperDoesNotRetryNonRetriableError(t *testing.T) {
	attempts := 0
	wantErr := errors.New("permanent error")

	err := DoWithSleeper(
		func() error {
			attempts++
			return wantErr
		},
		func(error) bool {
			return false
		},
		func(time.Duration) {
			t.Fatal("sleep should not be called")
		},
	)

	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if attempts != 1 {
		t.Fatalf("unexpected attempts count: got %d want 1", attempts)
	}
}
