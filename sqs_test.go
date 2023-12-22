package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
)

func TestGetSQSHandler(t *testing.T) {

	twoRecordEvent := events.SQSEvent{Records: []events.SQSMessage{
		{ReceiptHandle: "5a3e8884-4ff1-46f1-8617-b3f483a79956"},
		{ReceiptHandle: "2ecc59ae-ea1a-462a-8fca-d835858fc470"},
	}}

	testcases := []struct {
		name          string
		processRecord SQSRecordProcessor
		checkResult   func(t *testing.T, result events.SQSEventResponse)
		event         events.SQSEvent
	}{
		{
			name: "All messages processed",
			processRecord: func(ctx context.Context, record events.SQSMessage) error {
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{}}
				assert.Equal(t, expected, result)
			},
			event: twoRecordEvent,
		},
		{
			name: "Some messages fail",
			processRecord: func(ctx context.Context, record events.SQSMessage) error {
				if record.ReceiptHandle == "2ecc59ae-ea1a-462a-8fca-d835858fc470" {
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
			processRecord: func(ctx context.Context, record events.SQSMessage) error {
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
			processRecord: func(ctx context.Context, record events.SQSMessage) error {
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
			processRecord: func(ctx context.Context, record events.SQSMessage) error {
				if record.ReceiptHandle == "5a3e8884-4ff1-46f1-8617-b3f483a79956" {
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
			processRecord: func(ctx context.Context, record events.SQSMessage) error {
				return nil
			},
			checkResult: func(t *testing.T, result events.SQSEventResponse) {
				expected := events.SQSEventResponse{BatchItemFailures: []events.SQSBatchItemFailure{}}
				assert.Equal(t, expected, result)
			},
			event: events.SQSEvent{Records: []events.SQSMessage{
				{ReceiptHandle: "25209c2d-32e5-4117-9c09-dc4d3e954ade"},
			}},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
			defer cancel()

			handler := GetSQSHandler(tc.processRecord)
			logger := GetLogger(ctx)
			logger.Info("Start test")
			result, err := handler(ctx, tc.event)
			assert.Nil(t, err)
			tc.checkResult(t, result)
			logger.Info("End test")
		})
	}
}
