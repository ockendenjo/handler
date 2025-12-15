package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestStruct struct {
	ID string
}

func TestGetSQSHandler(t *testing.T) {

	twoRecordEvent := events.SQSEvent{Records: []events.SQSMessage{
		{ReceiptHandle: "5a3e8884-4ff1-46f1-8617-b3f483a79956", Body: `{"ID": "5"}`},
		{ReceiptHandle: "2ecc59ae-ea1a-462a-8fca-d835858fc470", Body: `{"ID": "2"}`},
	}}

	testcases := []struct {
		name          string
		processRecord SQSRecordProcessor[TestStruct]
		checkResult   func(t *testing.T, result events.SQSEventResponse)
		event         events.SQSEvent
	}{
		{
			name: "All messages processed",
			processRecord: func(ctx *Context, body TestStruct, _ map[string]events.SQSMessageAttribute) error {
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{}}
				assert.Equal(t, expected, result)
			},
			event: twoRecordEvent,
		},
		{
			name: "One routine panics",
			processRecord: func(ctx *Context, record TestStruct, _ map[string]events.SQSMessageAttribute) error {
				if record.ID == "2" {
					panic("oh no")
				}
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{
					{ItemIdentifier: "2ecc59ae-ea1a-462a-8fca-d835858fc470"},
				}}
				assert.Equal(t, expected, result)
			},
			event: twoRecordEvent,
		},
		{
			name: "Some messages fail",
			processRecord: func(ctx *Context, record TestStruct, _ map[string]events.SQSMessageAttribute) error {
				if record.ID == "2" {
					return errors.New("something bad happened")
				}
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{
					{ItemIdentifier: "2ecc59ae-ea1a-462a-8fca-d835858fc470"},
				}}
				assert.Equal(t, expected, result)
			},
			event: twoRecordEvent,
		},
		{
			name: "All messages fail",
			processRecord: func(ctx *Context, body TestStruct, _ map[string]events.SQSMessageAttribute) error {
				return errors.New("something bad happened")
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				errorMap := map[string]bool{}
				for _, failure := range result.BatchItemFailures {
					errorMap[failure.ItemIdentifier] = true
				}
				assert.True(t, errorMap["5a3e8884-4ff1-46f1-8617-b3f483a79956"])
				assert.True(t, errorMap["2ecc59ae-ea1a-462a-8fca-d835858fc470"])
			},
			event: twoRecordEvent,
		},
		{
			name: "Messages time-out",
			processRecord: func(ctx *Context, record TestStruct, _ map[string]events.SQSMessageAttribute) error {
				time.Sleep(10 * time.Second)
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				errorMap := map[string]bool{}
				for _, failure := range result.BatchItemFailures {
					errorMap[failure.ItemIdentifier] = true
				}
				assert.True(t, errorMap["5a3e8884-4ff1-46f1-8617-b3f483a79956"])
				assert.True(t, errorMap["2ecc59ae-ea1a-462a-8fca-d835858fc470"])
			},
			event: twoRecordEvent,
		},
		{
			name: "One message time-out",
			processRecord: func(ctx *Context, record TestStruct, _ map[string]events.SQSMessageAttribute) error {
				if record.ID == "5" {
					time.Sleep(10 * time.Second)
					return nil
				}
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{
					{ItemIdentifier: "5a3e8884-4ff1-46f1-8617-b3f483a79956"},
				}}
				assert.Equal(t, expected, result)
			},
			event: twoRecordEvent,
		},
		{
			name: "invoke with single record",
			processRecord: func(ctx *Context, record TestStruct, _ map[string]events.SQSMessageAttribute) error {
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{}}
				assert.Equal(t, expected, result)
			},
			event: events.SQSEvent{Records: []events.SQSMessage{
				{ReceiptHandle: "25209c2d-32e5-4117-9c09-dc4d3e954ade", Body: `{"ID": "2"}`},
			}},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(2*time.Second))
			defer cancel()

			handlerCtx := GetWithSuppressedLogging(ctx)

			w := &sqsWrapper{ProcessRecordFn: tc.processRecord}
			handler := GetSQSHandler(w, nil)
			logger := handlerCtx.GetLogger()
			logger.Info("Start test")
			result, err := handler(handlerCtx, tc.event)
			require.NoError(t, err)
			tc.checkResult(t, result)
			logger.Info("End test")
		})
	}
}

type sqsWrapper struct {
	ProcessRecordFn SQSRecordProcessor[TestStruct]
}

func (w *sqsWrapper) ProcessSQSEvent(ctx *Context, msg TestStruct, attributes map[string]events.SQSMessageAttribute) error {
	return w.ProcessRecordFn(ctx, msg, attributes)
}
