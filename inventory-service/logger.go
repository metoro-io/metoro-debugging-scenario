package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

type StructuredLogger struct {
	serviceName string
	output      io.Writer
}

type LogEntry struct {
	Timestamp   string                 `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	ServiceName string                 `json:"service_name"`
	TraceID     string                 `json:"trace_id,omitempty"`
	SpanID      string                 `json:"span_id,omitempty"`
	Message     string                 `json:"message"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

func NewStructuredLogger(serviceName string) *StructuredLogger {
	return &StructuredLogger{
		serviceName: serviceName,
		output:      os.Stdout,
	}
}

func (l *StructuredLogger) extractTraceInfo(ctx context.Context) (traceID, spanID string) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}
	return
}

func (l *StructuredLogger) log(ctx context.Context, level LogLevel, message string, fields map[string]interface{}) {
	traceID, spanID := l.extractTraceInfo(ctx)

	entry := LogEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Level:       level,
		ServiceName: l.serviceName,
		TraceID:     traceID,
		SpanID:      spanID,
		Message:     message,
		Fields:      fields,
	}

	data, _ := json.Marshal(entry)
	fmt.Fprintln(l.output, string(data))
}

func (l *StructuredLogger) Debug(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, LevelDebug, message, f)
}

func (l *StructuredLogger) Info(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, LevelInfo, message, f)
}

func (l *StructuredLogger) Warn(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, LevelWarn, message, f)
}

func (l *StructuredLogger) Error(ctx context.Context, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ctx, LevelError, message, f)
}

func (l *StructuredLogger) WithFields(fields map[string]interface{}) *LoggerWithFields {
	return &LoggerWithFields{
		logger: l,
		fields: fields,
	}
}

type LoggerWithFields struct {
	logger *StructuredLogger
	fields map[string]interface{}
}

func (lf *LoggerWithFields) Debug(ctx context.Context, message string) {
	lf.logger.Debug(ctx, message, lf.fields)
}

func (lf *LoggerWithFields) Info(ctx context.Context, message string) {
	lf.logger.Info(ctx, message, lf.fields)
}

func (lf *LoggerWithFields) Warn(ctx context.Context, message string) {
	lf.logger.Warn(ctx, message, lf.fields)
}

func (lf *LoggerWithFields) Error(ctx context.Context, message string) {
	lf.logger.Error(ctx, message, lf.fields)
}
