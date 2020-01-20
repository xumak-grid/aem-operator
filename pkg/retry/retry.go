package retry

import (
	"fmt"
	"time"
)

// ErrRetry is returned with the number of attempted retries.
type ErrRetry struct {
	n int
}

func (e *ErrRetry) Error() string {
	return fmt.Sprintf("additional failures after %d retries", e.n)
}

// IsRetryFailure checks if an error is a retry failure.
func IsRetryFailure(err error) bool {
	_, ok := err.(*ErrRetry)
	return ok
}

// ConditionFunc is a helper function for retries.
type ConditionFunc func() (bool, error)

// Retry attempts to retry a given function every interval until maxRetries.
// The interval is not affected by the duration of execution for f.
// Example: If interval is 3s, f takes 1s, another f will be called 2s later.
// However, if f takes longer than interval, it will be delayed.
func Retry(interval time.Duration, maxRetries int, f ConditionFunc) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for i := 0; ; i++ {
		ok, err := f()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i+1 == maxRetries {
			break
		}
		<-tick.C
	}
	return &ErrRetry{maxRetries}
}
