package repository

import (
	"fmt"
	"strings"
	"time"
)

// RetryWrite executes a write operation and retries if "database is locked" error occurs
// It uses a simple backoff strategy: 50ms, 100ms, 200ms
func RetryWrite(fn func() error, maxRetries int) error {
	for i := 0; i <= maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if "database is locked"
		// SQLite error message for locked database
		if strings.Contains(err.Error(), "database is locked") {
			if i == maxRetries {
				return fmt.Errorf("max retries exceeded: %v", err)
			}

			// Simple backoff: 50ms, 100ms, 200ms...
			backoff := time.Duration(50*(i+1)) * time.Millisecond
			time.Sleep(backoff)
			continue
		}

		// Other error, don't retry
		return err
	}
	return fmt.Errorf("max retries execution error")
}
