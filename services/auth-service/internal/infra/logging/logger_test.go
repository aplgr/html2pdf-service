package logging

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestSetLogLevelAndStructuredLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	SetLoggerForTest(zerolog.New(buf))

	SetLogLevel("invalid-level") // should fallback to info without panic
	Info("hello", "k", "v", "dangling")
	Warn("warn", "n", 1)
	Error("err", "ok", true)

	if buf.Len() == 0 {
		t.Fatalf("expected log output")
	}
}
