package logging

import (
	"path/filepath"
	"testing"
)

func TestInitLoggerAndSetLogLevelFallback(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "renderer.log")
	InitLogger(logFile, 1, 1, 1, false, "invalid")
	SetLogLevel("invalid")
	Info("hello", "k", "v")
	Warn("warn")
	Error("error")
}
