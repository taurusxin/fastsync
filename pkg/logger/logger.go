package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	mu    sync.Mutex
	out   io.Writer
	level Level
}

func New(out io.Writer, level Level) *Logger {
	return &Logger{
		out:   out,
		level: level,
	}
}

func (l *Logger) Output(level Level, msg string) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.out, "[%s] [%s] %s\n", timestamp, level.String(), msg)
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.Output(LevelInfo, fmt.Sprintf(format, v...))
}

func (l *Logger) Warn(format string, v ...interface{}) {
	l.Output(LevelWarn, fmt.Sprintf(format, v...))
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.Output(LevelError, fmt.Sprintf(format, v...))
}

// Global logger instance
var std = New(os.Stdout, LevelInfo)

func SetGlobal(l *Logger) {
	std = l
}

func Info(format string, v ...interface{}) {
	std.Info(format, v...)
}

func Warn(format string, v ...interface{}) {
	std.Warn(format, v...)
}

func Error(format string, v ...interface{}) {
	std.Error(format, v...)
}
