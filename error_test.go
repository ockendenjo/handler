package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsErrorRetryable(t *testing.T) {

	testcases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "should return false for non-retryable error",
			err:      errors.New("test"),
			expected: false,
		},
		{
			name:     "should return true for retryable error",
			err:      &testRetryableError{},
			expected: true,
		},
		{
			name:     "should return false for wrapped non-retryable error",
			err:      fmt.Errorf("outer: %w", errors.New("test")),
			expected: false,
		},
		{
			name:     "should return true for wrapped retryable error",
			err:      fmt.Errorf("outer: %w", &testRetryableError{}),
			expected: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			isRetryableError := IsErrorRetryable(tc.err)
			assert.Equal(t, tc.expected, isRetryableError)
		})
	}
}

type testRetryableError struct {
}

func (t *testRetryableError) Error() string {
	return "error for testing"
}

func (t *testRetryableError) IsRetryable() bool {
	return true
}
