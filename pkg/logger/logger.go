// Package logger provides a minimal structured logger. H is a shorthand for
// attaching key/value context to a log line.
package logger

import (
	"encoding/json"
	"log"
	"os"
)

type H map[string]any

type Logger interface {
	Info(msg string, fields H)
	Error(msg string, fields H)
}

type consoleLogger struct {
	std *log.Logger
}

func NewConsoleLogger() Logger {
	return &consoleLogger{std: log.New(os.Stdout, "", log.LstdFlags)}
}

func (l *consoleLogger) Info(msg string, fields H) {
	l.print("INFO", msg, fields)
}

func (l *consoleLogger) Error(msg string, fields H) {
	l.print("ERROR", msg, fields)
}

func (l *consoleLogger) print(level, msg string, fields H) {
	if fields == nil {
		l.std.Printf("[%s] %s", level, msg)
		return
	}
	b, err := json.Marshal(fields)
	if err != nil {
		l.std.Printf("[%s] %s %v", level, msg, fields)
		return
	}
	l.std.Printf("[%s] %s %s", level, msg, string(b))
}
