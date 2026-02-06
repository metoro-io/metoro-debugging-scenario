package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
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
	Message     string                 `json:"message"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

func NewStructuredLogger(serviceName string) *StructuredLogger {
	return &StructuredLogger{
		serviceName: serviceName,
		output:      os.Stdout,
	}
}

func (l *StructuredLogger) log(ctx context.Context, level LogLevel, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Level:       level,
		ServiceName: l.serviceName,
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
