package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
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

func (l Level) ColorString() string {
	switch l {
	case LevelInfo:
		return color.GreenString("INFO")
	case LevelWarn:
		return color.YellowString("WARN")
	case LevelError:
		return color.RedString("ERROR")
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	level  Level
	prefix string
}

func New(out io.Writer, level Level, prefix string) *Logger {
	return &Logger{
		out:    out,
		level:  level,
		prefix: prefix,
	}
}

func (l *Logger) Output(level Level, msg string) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Determine if we should use colors.
	useColor := false
	if f, ok := l.out.(*os.File); ok {
		if f == os.Stdout || f == os.Stderr {
			useColor = true
		}
	}

	var levelStr string
	var prefixStr string

	if useColor {
		levelStr = level.ColorString()
		if l.prefix != "" {
			prefixStr = fmt.Sprintf("[%s] ", color.CyanString(l.prefix))
		}
	} else {
		levelStr = level.String()
		if l.prefix != "" {
			prefixStr = fmt.Sprintf("[%s] ", l.prefix)
		}
	}

	fmt.Fprintf(l.out, "[%s] [%s] %s%s\n", timestamp, levelStr, prefixStr, msg)
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
var std = New(os.Stdout, LevelInfo, "Main")

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
