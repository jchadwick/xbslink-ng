// Package logging provides a leveled logger with colored output and timestamps.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents the logging level.
type Level int

const (
	// LevelError logs only errors.
	LevelError Level = iota
	// LevelWarn logs warnings and errors.
	LevelWarn
	// LevelInfo logs info, warnings, and errors.
	LevelInfo
	// LevelDebug logs debug messages and above.
	LevelDebug
	// LevelTrace logs everything including trace-level details.
	LevelTrace
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelWarn:
		return "WARN"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	case LevelTrace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// Logger provides leveled logging with optional color support.
type Logger struct {
	level     Level
	output    io.Writer
	useColor  bool
	mu        sync.Mutex
	timestamp string // format string for timestamps
}

// NewLogger creates a new logger with the specified level.
// Color output is automatically enabled if writing to a terminal.
func NewLogger(level Level) *Logger {
	return &Logger{
		level:     level,
		output:    os.Stdout,
		useColor:  isTTY(os.Stdout),
		timestamp: "2006-01-02 15:04:05",
	}
}

// SetOutput sets the output writer for the logger.
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
	// Re-evaluate color support based on new output
	if f, ok := w.(*os.File); ok {
		l.useColor = isTTY(f)
	} else {
		l.useColor = false
	}
}

// SetColorEnabled explicitly enables or disables color output.
func (l *Logger) SetColorEnabled(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.useColor = enabled
}

// SetLevel changes the logging level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current logging level.
func (l *Logger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// log writes a log message at the specified level.
func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level > l.level {
		return
	}

	timestamp := time.Now().Format(l.timestamp)
	message := fmt.Sprintf(format, args...)

	var levelStr string
	var colorCode string

	switch level {
	case LevelError:
		levelStr = "ERROR"
		colorCode = colorRed
	case LevelWarn:
		levelStr = "WARN"
		colorCode = colorYellow
	case LevelInfo:
		levelStr = "INFO"
		colorCode = colorGreen
	case LevelDebug:
		levelStr = "DEBUG"
		colorCode = colorCyan
	case LevelTrace:
		levelStr = "TRACE"
		colorCode = colorGray
	}

	if l.useColor {
		fmt.Fprintf(l.output, "%s [%s%s%s]  %s\n", timestamp, colorCode, levelStr, colorReset, message)
	} else {
		fmt.Fprintf(l.output, "%s [%s]  %s\n", timestamp, levelStr, message)
	}
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Trace logs a trace message (most verbose).
func (l *Logger) Trace(format string, args ...interface{}) {
	l.log(LevelTrace, format, args...)
}

// Stats logs a statistics line with special formatting.
func (l *Logger) Stats(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format(l.timestamp)
	message := fmt.Sprintf(format, args...)

	if l.useColor {
		fmt.Fprintf(l.output, "%s [%sSTATS%s] %s\n", timestamp, colorBold, colorReset, message)
	} else {
		fmt.Fprintf(l.output, "%s [STATS] %s\n", timestamp, message)
	}
}

// ParseLevel parses a string into a Level.
// Valid values: error, warn, info, debug, trace (case-insensitive).
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error":
		return LevelError, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "info":
		return LevelInfo, nil
	case "debug":
		return LevelDebug, nil
	case "trace":
		return LevelTrace, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level %q: must be error, warn, info, debug, or trace", s)
	}
}

// isTTY checks if the given file is a terminal.
func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
