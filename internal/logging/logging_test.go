package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger(LevelInfo)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.GetLevel() != LevelInfo {
		t.Errorf("expected level INFO, got %v", logger.GetLevel())
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		level         Level
		expectError   bool
		expectWarn    bool
		expectInfo    bool
		expectDebug   bool
		expectTrace   bool
	}{
		{LevelError, true, false, false, false, false},
		{LevelWarn, true, true, false, false, false},
		{LevelInfo, true, true, true, false, false},
		{LevelDebug, true, true, true, true, false},
		{LevelTrace, true, true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(tt.level)
			logger.SetOutput(&buf)
			logger.SetColorEnabled(false)

			logger.Error("error msg")
			logger.Warn("warn msg")
			logger.Info("info msg")
			logger.Debug("debug msg")
			logger.Trace("trace msg")

			output := buf.String()

			checkContains(t, output, "ERROR", "error msg", tt.expectError)
			checkContains(t, output, "WARN", "warn msg", tt.expectWarn)
			checkContains(t, output, "INFO", "info msg", tt.expectInfo)
			checkContains(t, output, "DEBUG", "debug msg", tt.expectDebug)
			checkContains(t, output, "TRACE", "trace msg", tt.expectTrace)
		})
	}
}

func checkContains(t *testing.T, output, level, msg string, shouldContain bool) {
	t.Helper()
	contains := strings.Contains(output, level) && strings.Contains(output, msg)
	if shouldContain && !contains {
		t.Errorf("expected output to contain [%s] %s", level, msg)
	}
	if !shouldContain && contains {
		t.Errorf("expected output NOT to contain [%s] %s", level, msg)
	}
}

func TestLogger_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo)
	logger.SetOutput(&buf)
	logger.SetColorEnabled(false)

	logger.Info("test message")

	output := buf.String()

	// Check format: "YYYY-MM-DD HH:MM:SS [LEVEL]  message\n"
	if !strings.Contains(output, "[INFO]") {
		t.Error("expected [INFO] in output")
	}
	if !strings.Contains(output, "test message") {
		t.Error("expected message in output")
	}
	// Check for timestamp-like pattern (basic check)
	if len(output) < 20 {
		t.Error("output too short for expected format")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := NewLogger(LevelInfo)

	if logger.GetLevel() != LevelInfo {
		t.Errorf("expected INFO, got %v", logger.GetLevel())
	}

	logger.SetLevel(LevelDebug)

	if logger.GetLevel() != LevelDebug {
		t.Errorf("expected DEBUG, got %v", logger.GetLevel())
	}
}

func TestLogger_SetOutput(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger := NewLogger(LevelInfo)
	logger.SetColorEnabled(false)

	logger.SetOutput(&buf1)
	logger.Info("message1")

	logger.SetOutput(&buf2)
	logger.Info("message2")

	if !strings.Contains(buf1.String(), "message1") {
		t.Error("expected message1 in buf1")
	}
	if strings.Contains(buf1.String(), "message2") {
		t.Error("expected message2 NOT in buf1")
	}
	if !strings.Contains(buf2.String(), "message2") {
		t.Error("expected message2 in buf2")
	}
}

func TestLogger_SetColorEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo)
	logger.SetOutput(&buf)

	// Disable colors
	logger.SetColorEnabled(false)
	logger.Info("no color")
	
	output := buf.String()
	if strings.Contains(output, "\033[") {
		t.Error("expected no ANSI codes when color disabled")
	}

	// Enable colors
	buf.Reset()
	logger.SetColorEnabled(true)
	logger.Info("with color")

	output = buf.String()
	if !strings.Contains(output, "\033[") {
		t.Error("expected ANSI codes when color enabled")
	}
}

func TestLogger_Stats(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelError) // Even at ERROR level
	logger.SetOutput(&buf)
	logger.SetColorEnabled(false)

	logger.Stats("stats message")

	output := buf.String()
	if !strings.Contains(output, "[STATS]") {
		t.Error("expected [STATS] in output")
	}
	if !strings.Contains(output, "stats message") {
		t.Error("expected stats message in output")
	}
}

func TestLogger_FormatArgs(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelInfo)
	logger.SetOutput(&buf)
	logger.SetColorEnabled(false)

	logger.Info("count: %d, name: %s", 42, "test")

	output := buf.String()
	if !strings.Contains(output, "count: 42") {
		t.Error("expected formatted count")
	}
	if !strings.Contains(output, "name: test") {
		t.Error("expected formatted name")
	}
}

func TestParseLevel_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"error", LevelError},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"info", LevelInfo},
		{"debug", LevelDebug},
		{"trace", LevelTrace},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, level, tt.expected)
			}
		})
	}
}

func TestParseLevel_CaseInsensitive(t *testing.T) {
	tests := []string{"ERROR", "Error", "ErRoR", "  error  ", "ERROR "}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			level, err := ParseLevel(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != LevelError {
				t.Errorf("ParseLevel(%q) = %v, want LevelError", input, level)
			}
		})
	}
}

func TestParseLevel_Invalid(t *testing.T) {
	invalids := []string{"invalid", "verbose", "", "123", "ERRORS"}

	for _, input := range invalids {
		t.Run(input, func(t *testing.T) {
			_, err := ParseLevel(input)
			if err == nil {
				t.Errorf("expected error for ParseLevel(%q)", input)
			}
		})
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelError, "ERROR"},
		{LevelWarn, "WARN"},
		{LevelInfo, "INFO"},
		{LevelDebug, "DEBUG"},
		{LevelTrace, "TRACE"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("Level(%d).String() = %s, want %s", tt.level, tt.level.String(), tt.expected)
			}
		})
	}
}

func TestLogger_ConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LevelTrace)
	logger.SetOutput(&buf)
	logger.SetColorEnabled(false)

	done := make(chan bool)

	// Multiple goroutines writing
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				logger.Info("goroutine %d: message %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that we got output (no crashes)
	if buf.Len() == 0 {
		t.Error("expected some output")
	}
}
