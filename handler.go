package handler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Handler[T interface{}, U interface{}] func(ctx *Context, event T) (U, error)
type BasicHandler[T interface{}, U interface{}] func(ctx context.Context, event T) (U, error)

func withLogger[T interface{}, U interface{}](handlerFunc Handler[T, U], logWriter *io.Writer) BasicHandler[T, U] {
	return func(ctx context.Context, event T) (U, error) {
		var logger *Logger
		if logWriter == nil {
			logger = getStoryLoggerWithWriter(os.Stdout)
		} else {
			logger = getStoryLoggerWithWriter(*logWriter)
		}

		handlerCtx := &Context{Context: ctx, storyLogger: logger}

		response, err := handlerFunc(handlerCtx, event)
		if err != nil {
			logger.Error("lambda execution failed", "error", err.Error())
		}

		handlerCtx.finalize()
		return response, err
	}
}

func getJSONSlogFromWriter(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}

func getStoryLoggerWithWriter(w io.Writer) *Logger {
	traceId := os.Getenv("_X_AMZN_TRACE_ID")
	logger := getJSONSlogFromWriter(w)

	if traceId != "" {
		parts := strings.Split(traceId, ";")
		if len(parts) > 0 {
			logger = logger.With("trace_id", strings.Replace(parts[0], "Root=", "", 1))
		}
	}

	storyLogger := newStoryLogger(logger)
	storyLogger.combinedMode = true
	return storyLogger
}

func Get(ctx context.Context) *Context {
	return &Context{Context: ctx}
}

func GetJSONTestLogger(ctx context.Context) (*Context, func()) {
	storyLogger := getStoryLoggerWithWriter(os.Stdout)
	hCtx := &Context{Context: ctx, storyLogger: storyLogger}
	return hCtx, hCtx.finalize
}

func GetWithSuppressedLogging(ctx context.Context) *Context {
	slogger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return GetWithSlogLogger(ctx, slogger)
}

func GetWithSlogLogger(ctx context.Context, slogger *slog.Logger) *Context {
	storyLogger := newStoryLogger(slogger)
	storyLogger.combinedMode = true
	return &Context{Context: ctx, storyLogger: storyLogger}
}
