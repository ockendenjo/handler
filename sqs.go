package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

type SQSRecordProcessor[T any] func(ctx *Context, genericType T, attributes map[string]events.SQSMessageAttribute) error

type SQSHandlerStruct[T any] interface {
	ProcessSQSEvent(ctx *Context, genericType T, attributes map[string]events.SQSMessageAttribute) error
}

type SQSHandler = Handler[events.SQSEvent, events.SQSEventResponse]

type LoggerParams struct {
	params map[string]any
}

func (lp *LoggerParams) Add(key string, value any) {
	lp.params[key] = value
}

func NewLoggerParams() *LoggerParams {
	return &LoggerParams{params: make(map[string]any)}
}

// GetSQSHandler returns a lambda handler that will process each SQS message in parallel using the provided processRecord function
func GetSQSHandler[T any](sqsHandlerIface SQSHandlerStruct[T], addLoggerParams func(lp *LoggerParams, t T)) Handler[events.SQSEvent, events.SQSEventResponse] {

	logInputEvent := GetEnv("LOG_INPUT_EVENT") == "true"

	process := func(ctx *Context, record events.SQSMessage, successChannel chan bool) {
		var genericType T
		logger := ctx.GetLogger()

		defer func() {
			if r := recover(); r != nil {
				strStack := getStackTraceAsSlice(debug.Stack())
				logger.With("panicStack", strStack).Errorf("Goroutine panicked: %v", r)
				successChannel <- false
			}
		}()

		err := json.Unmarshal([]byte(record.Body), &genericType)
		if err != nil {
			logger.Error("JSON unmarshal returned error", "error", err.Error(), "body", record.Body)
			successChannel <- false
			return
		}

		if logInputEvent {
			logger = logger.With("inputEvent", genericType)
		}

		lp := NewLoggerParams()
		if addLoggerParams != nil {
			addLoggerParams(lp, genericType)
		}
		for k, v := range lp.params {
			logger = logger.With(k, v)
		}

		err = sqsHandlerIface.ProcessSQSEvent(ctx, genericType, record.MessageAttributes)

		if err != nil {
			logger.AddParam("body", record.Body)
			if IsErrorRetryable(err) {
				logger.Infof("Processing returned error: %s", err.Error())
			} else {
				logger.Errorf("Processing returned error: %s", err.Error())
			}
			successChannel <- false
			return
		}
		successChannel <- true
	}

	return func(ctx *Context, event events.SQSEvent) (events.SQSEventResponse, error) {
		ctx.GetLogger().disableOutput() //Each SQS message will log its own story

		deadline, hasDeadline := ctx.Deadline()
		if !hasDeadline {
			return events.SQSEventResponse{}, errors.New("context must have a deadline set")
		}
		deadline = deadline.Add(-500 * time.Millisecond)
		subCtx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()

		//Process each SQS message in its own go routine
		routines := []*routineData[events.SQSMessage]{}
		for _, record := range event.Records {
			c := make(chan bool)

			routineCtx, closeLog := ctx.Split(subCtx)
			data := routineData[events.SQSMessage]{
				SuccessChannel: c,
				handlerCtx:     routineCtx,
				closeLog:       closeLog,
				Record:         record,
				TimeoutTimer:   time.NewTimer(time.Until(deadline)),
			}
			routines = append(routines, &data)
			go process(routineCtx, record, c)
		}

		//For each go routine, start another routine to wait for the result or the timeout
		wg := sync.WaitGroup{}
		for _, routine := range routines {
			wg.Go(asyncWaitForResult(routine))
		}

		//Collect the failures
		wg.Wait()
		failures := []events.SQSBatchItemFailure{}
		for _, r := range routines {
			if r.timedOut {
				r.handlerCtx.GetLogger().Info("Message processing timed out")
			}

			if r.failed || r.timedOut {
				failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: r.Record.ReceiptHandle})
				r.handlerCtx.GetLogger().Info("Message returned to queue for retry")
			}
			r.closeLog()
		}

		return events.SQSEventResponse{BatchItemFailures: failures}, nil
	}
}

func getStackTraceAsSlice(stack []byte) []string {
	byteParts := bytes.Split(stack, []byte("\n"))
	strParts := make([]string, 0, len(byteParts))
	for _, part := range byteParts {
		strPart := string(bytes.TrimSpace(part))
		if strPart != "" {
			strParts = append(strParts, strPart)
		}
	}
	return strParts
}

func asyncWaitForResult[T any](routine *routineData[T]) func() {
	return func() {
		select {
		case success := <-routine.SuccessChannel:
			routine.TimeoutTimer.Stop()
			if !success {
				routine.failed = true
			}
		case <-routine.TimeoutTimer.C:
			routine.timedOut = true
		}
	}
}

type routineData[T any] struct {
	SuccessChannel chan bool
	Record         T
	//Need a timer for each goroutine because the channel only receives one value
	TimeoutTimer *time.Timer
	failed       bool
	timedOut     bool
	handlerCtx   *Context
	closeLog     func()
}
