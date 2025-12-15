package handler

import (
	"context"
	"log/slog"
)

type Context struct {
	context.Context
	storyLogger *Logger
	metrics     []*MetricBuilder
}

func (h *Context) GetLogger() *Logger {
	if h.storyLogger != nil {
		return h.storyLogger
	}
	return newStoryLogger(slog.Default())
}

func (h *Context) Split(ctx context.Context) (*Context, func()) {
	logger := h.GetLogger()
	splitCtx := &Context{
		Context:     ctx,
		storyLogger: newStoryLogger(logger.slogger),
	}
	splitCtx.storyLogger.combinedMode = true
	deferFn := func() {
		splitCtx.finalize()
	}
	return splitCtx, deferFn
}

func (h *Context) finalize() {
	h.addMetricsToLogging()
	h.storyLogger.Log()
}
