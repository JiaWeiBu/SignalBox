package signalbox

import (
	"fmt"
	"time"
)

// SignalEvent describes one runtime occurrence of a SignalDefinition.
type SignalEvent struct {
	Code      SignalCode     `json:"code"`
	Message   string         `json:"message"`
	Source    string         `json:"source"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// NewSignalEvent creates an event with the current UTC time and a copy of payload's top-level map.
func NewSignalEvent(code SignalCode, message, source string, payload map[string]any) (SignalEvent, error) {
	return NewSignalEventAt(code, message, source, time.Now().UTC(), payload)
}

// NewSignalEventAt creates an event with an explicit timestamp, useful when preserving event time.
func NewSignalEventAt(code SignalCode, message, source string, timestamp time.Time, payload map[string]any) (SignalEvent, error) {
	parsed, err := ParseSignalCode(code.String())
	if err != nil {
		return SignalEvent{}, err
	}
	return SignalEvent{
		Code:      parsed,
		Message:   message,
		Source:    source,
		Timestamp: timestamp,
		Payload:   clonePayload(payload),
	}, nil
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	clone := make(map[string]any, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
}

// SignalError adapts an event to Go's error interface when a caller needs an error return.
type SignalError struct {
	Event SignalEvent
}

func (e *SignalError) Error() string {
	if e == nil {
		return "signalbox: <nil> SignalError"
	}
	if e.Event.Message == "" {
		return e.Event.Code.String()
	}
	return fmt.Sprintf("%s: %s", e.Event.Code, e.Event.Message)
}
