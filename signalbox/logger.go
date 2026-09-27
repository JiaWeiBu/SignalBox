package signalbox

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Logger writes SignalEvents as one JSON object per line.
type Logger struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewLogger creates a structured JSON logger backed by writer.
func NewLogger(writer io.Writer) *Logger { return &Logger{writer: writer} }

// Emit validates and writes an event as structured JSON. Calls on one Logger are serialized.
func (logger *Logger) Emit(event SignalEvent) error {
	if logger == nil || logger.writer == nil {
		return fmt.Errorf("signalbox: logger has no output writer")
	}
	components, err := DecodeSignalCode(event.Code)
	if err != nil {
		return err
	}
	record := struct {
		Code      SignalCode     `json:"code"`
		Severity  uint8          `json:"severity"`
		Message   string         `json:"message"`
		Source    string         `json:"source"`
		Timestamp time.Time      `json:"timestamp"`
		Payload   map[string]any `json:"payload,omitempty"`
	}{
		Code:      event.Code,
		Severity:  uint8(components.Severity),
		Message:   event.Message,
		Source:    event.Source,
		Timestamp: event.Timestamp,
		Payload:   event.Payload,
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if err := json.NewEncoder(logger.writer).Encode(record); err != nil {
		return fmt.Errorf("signalbox: encode event: %w", err)
	}
	return nil
}

var defaultLogger = NewLogger(os.Stderr)

// Emit writes an event as structured JSON through Go's standard logger output.
func Emit(event SignalEvent) error { return defaultLogger.Emit(event) }

// NewStderrLogger creates a logger that writes JSON events to stderr.
func NewStderrLogger() *Logger { return NewLogger(os.Stderr) }
