package handler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithLogger(t *testing.T) {

	type testCase[T interface{}, U interface{}] struct {
		handler     Handler[T, U]
		checkResult func(t *testing.T, output U, err error)
		name        string
	}

	testCases := []testCase[inputEvent, outputEvent]{
		{
			name: "Handler returns result",
			handler: func(ctx *Context, event inputEvent) (outputEvent, error) {
				return outputEvent{Bar: 1}, nil
			},
			checkResult: func(t *testing.T, output outputEvent, err error) {
				require.NoError(t, err)
				assert.Equal(t, outputEvent{Bar: 1}, output)
			},
		},
		{
			name: "Handler returns error",
			handler: func(ctx *Context, event inputEvent) (outputEvent, error) {
				return outputEvent{}, errors.New("something bad happened")
			},
			checkResult: func(t *testing.T, output outputEvent, err error) {
				require.Error(t, err)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wrappedHandler := withLogger(tc.handler, nil)
			output, err := wrappedHandler(t.Context(), inputEvent{Foo: 1})
			tc.checkResult(t, output, err)
		})
	}
}

type inputEvent struct {
	Foo int
}

type outputEvent struct {
	Bar int
}
