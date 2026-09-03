package logger

import (
	"fmt"
	"log"
	"time"
)

// Level defines log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging.
type Logger struct {
	level  Level
	prefix string
}

// New creates a new logger.
func New(level Level, prefix string) *Logger {
	return &Logger{level: level, prefix: prefix}
}

func (l *Logger) log(level Level, levelStr, msg string, args ...any) {
	if level < l.level {
		return
	}
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z")
	formatted := fmt.Sprintf(msg, args...)
	if l.prefix != "" {
		log.Printf("%s [%s] %s: %s", timestamp, levelStr, l.prefix, formatted)
	} else {
		log.Printf("%s [%s] %s", timestamp, levelStr, formatted)
	}
}

func (l *Logger) Debug(msg string, args ...any) { l.log(LevelDebug, "DEBUG", msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.log(LevelInfo, "INFO", msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.log(LevelWarn, "WARN", msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.log(LevelError, "ERROR", msg, args...) }

// Global logger instance
var defaultLogger = New(LevelInfo, "")

func SetDefault(l *Logger) { defaultLogger = l }
func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }
