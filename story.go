package handler

import (
	"fmt"
	"log/slog"
	"strings"
)

func newStoryLogger(logger *slog.Logger) *Logger {
	return &Logger{
		slogger:    logger,
		stages:     make([]string, 0),
		params:     make(map[any]any),
		lineParams: make(map[any]any),
	}
}

type Logger struct {
	slogger      *slog.Logger
	stages       []string
	params       map[any]any
	lineParams   map[any]any
	disabled     bool
	errorLevel   bool
	combinedMode bool
}

func (s *Logger) DisableCombinedMode() {
	s.combinedMode = false
}

// AddStage adds a stage to the logging story
//
// description should be in the form <noun> <verb> <other words>. For example:
// Invocation returned error, OR Validation succeeded
func (s *Logger) AddStage(description string) *Logger {
	s.stages = append(s.stages, description)
	return s
}

func (s *Logger) AddStageIfNoError(description string, err error) error {
	if err == nil {
		s.AddStage(description)
	}
	return err
}

func (s *Logger) AddParam(key any, value any) *Logger {
	s.params[key] = value
	return s
}

func (s *Logger) addParams(keyValues ...any) *Logger {
	for i := 0; i < len(keyValues); i += 2 {
		key := keyValues[i]
		var value any
		if i+1 < len(keyValues) {
			value = keyValues[i+1]
		} else {
			value = "FIXME - odd number of params"
		}
		s.AddParam(key, value)
	}
	return s
}

func (s *Logger) disableOutput() {
	s.disabled = true
}

func (s *Logger) Log() {
	if s.disabled {
		return
	}
	if !s.combinedMode {
		return
	}
	if len(s.stages) == 0 && len(s.params) == 0 {
		return
	}

	logger := s.slogger
	for k, v := range s.params {
		logger = logger.With(k, v)
	}
	if len(s.stages) > 0 {
		logger = logger.With("stages", s.stages)
	}
	description := strings.Join(s.stages, "; ")
	if len(description) > 100 {
		description = description[:100] + "..."
	}

	if s.errorLevel {
		logger.Error(description)
	} else {
		logger.Info(description)
	}
}

func (s *Logger) Info(msg string, args ...any) {
	if s.combinedMode {
		s.legacyLog(msg, args...)
	} else {
		slogger := s.slogger
		for k, v := range s.params {
			slogger = slogger.With(k, v)
		}
		slogger.Info(msg, args...)
	}
}

func (s *Logger) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.Info(msg)
}

func (s *Logger) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	s.Error(msg)
}

func (s *Logger) Debug(msg string, args ...any) {
	if s.combinedMode {
		//Don't do anything
	} else {
		s.slogger.Debug(msg, args...)
	}
}

func (s *Logger) Warn(msg string, args ...any) {
	if s.combinedMode {
		s.legacyLog(msg, args...)
	} else {
		s.slogger.Warn(msg, args...)
	}
}

func (s *Logger) Error(msg string, args ...any) {
	if s.combinedMode {
		s.legacyLog(msg, args...)
		s.errorLevel = true
	} else {
		s.slogger.Error(msg, args...)
	}
}

func formatMsgAndArgs(msg string, args map[any]any) string {
	if len(args) == 0 {
		return msg
	}

	var builder strings.Builder
	builder.WriteString(msg)
	builder.WriteString("; ")

	i := 0
	for k, v := range args {
		if i > 0 {
			builder.WriteString("; ")
		}
		builder.WriteString(fmt.Sprintf("%v='%v'", k, v))
		i++
	}

	return builder.String()
}

func (s *Logger) legacyLog(msg string, args ...any) {
	if args != nil {
		s.WithLineParams(args...)
	}
	s.AddStage(formatMsgAndArgs(msg, s.lineParams))
	s.lineParams = make(map[any]any)
}

func (s *Logger) With(args ...any) *Logger {
	s.addParams(args...)
	return s
}

// WithLineParams adds common key-value pairs which will be appended to the end of the next line which is logged.
// For example:
//
// `message; a=1; b=foo`
func (s *Logger) WithLineParams(args ...any) *Logger {
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		var value any
		if i+1 < len(args) {
			value = args[i+1]
		} else {
			value = key
			key = "FIXME_ODD_NUM_PARAMS"
		}
		s.lineParams[key] = value
	}
	return s
}
