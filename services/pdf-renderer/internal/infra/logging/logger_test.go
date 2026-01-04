package logging

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// setupTestLogger configures a logger with a custom writer for tests
func setupTestLogger(output *bytes.Buffer, level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	logger := zerolog.New(output).With().Timestamp().Logger().Level(lvl)

	// Manually set the logger (workaround because `utils.logger` is unexported)
	SetLoggerForTest(logger)
}

func TestInfoLogging(t *testing.T) {
	var buf bytes.Buffer
	setupTestLogger(&buf, "info")

	Info("test message", "foo", 42, "bar", true)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "test message") {
		t.Error("Expected log message not found in output")
	}
	if !strings.Contains(logOutput, `"foo":42`) || !strings.Contains(logOutput, `"bar":true`) {
		t.Error("Expected key-value pairs not found in output")
	}
}

func TestWarnLogging(t *testing.T) {
	var buf bytes.Buffer
	setupTestLogger(&buf, "warn")

	Warn("something odd", "code", 99)

	if !strings.Contains(buf.String(), "something odd") || !strings.Contains(buf.String(), `"code":99`) {
		t.Error("Warn log output missing expected content")
	}
}

func TestErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	setupTestLogger(&buf, "error")

	Error("error occurred", "fatal", false)

	if !strings.Contains(buf.String(), "error occurred") || !strings.Contains(buf.String(), `"fatal":false`) {
		t.Error("Error log output missing expected content")
	}
}

func TestSetLogLevel(t *testing.T) {
	var buf bytes.Buffer
	setupTestLogger(&buf, "warn")

	SetLogLevel("info")
	Info("should be visible")

	if !strings.Contains(buf.String(), "should be visible") {
		t.Error("Expected info log after SetLogLevel not found")
	}
}
