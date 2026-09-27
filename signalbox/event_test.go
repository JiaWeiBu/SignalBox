package signalbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSignalEventCreationAndTimestampPreservation(t *testing.T) {
	code := SignalCode("3C200001")
	payload := map[string]any{"requested": 6, "available": 4}
	event, err := NewSignalEvent(code, "Insufficient capacity", "switchyard-main", payload)
	if err != nil {
		t.Fatal(err)
	}
	if event.Timestamp.IsZero() {
		t.Fatal("NewSignalEvent returned a zero timestamp")
	}
	payload["requested"] = 9
	if got := event.Payload["requested"]; got != 6 {
		t.Errorf("event payload changed with source map: got %v, want 6", got)
	}

	wantTime := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.FixedZone("test", 3600))
	preserved, err := NewSignalEventAt(code, "Historical event", "switchyard-main", wantTime, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !preserved.Timestamp.Equal(wantTime) {
		t.Errorf("timestamp = %s, want %s", preserved.Timestamp, wantTime)
	}
	if _, err := NewSignalEvent("invalid", "bad", "source", nil); !errors.Is(err, ErrMalformedCode) {
		t.Errorf("invalid code error = %v", err)
	}
}

func TestSignalEventJSONAndSignalError(t *testing.T) {
	event, err := NewSignalEvent("3C200001", "Insufficient capacity", "switchyard-main", map[string]any{"requested": 6})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	logger := NewLogger(&output)
	if err := logger.Emit(event); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Code      SignalCode     `json:"code"`
		Severity  uint8          `json:"severity"`
		Message   string         `json:"message"`
		Source    string         `json:"source"`
		Timestamp time.Time      `json:"timestamp"`
		Payload   map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("log output is not JSON: %v", err)
	}
	if record.Code != event.Code || record.Severity != 3 || record.Message != event.Message || record.Source != event.Source {
		t.Errorf("structured record missing event fields: %#v", record)
	}
	if got := record.Payload["requested"]; got != float64(6) {
		t.Errorf("payload requested = %v, want 6", got)
	}
	if record.Timestamp.IsZero() {
		t.Error("structured record omitted event timestamp")
	}

	signalErr := &SignalError{Event: event}
	var _ error = signalErr
	if got, want := signalErr.Error(), "3C200001: Insufficient capacity"; got != want {
		t.Errorf("SignalError.Error() = %q, want %q", got, want)
	}
	if signalErr.Event.Code != event.Code || signalErr.Event.Payload["requested"] != 6 {
		t.Errorf("SignalError did not retain its event: %#v", signalErr.Event)
	}
}

func TestLoggerRejectsMalformedEventAndNilWriter(t *testing.T) {
	if err := NewLogger(&bytes.Buffer{}).Emit(SignalEvent{Code: "bad"}); !errors.Is(err, ErrMalformedCode) {
		t.Errorf("malformed event error = %v", err)
	}
	if err := NewLogger(nil).Emit(SignalEvent{Code: "3C200001"}); err == nil {
		t.Error("nil writer should return an error")
	}
}
