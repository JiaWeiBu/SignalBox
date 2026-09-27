package signalbox

import (
	"errors"
	"fmt"
)

const (
	// CodeLength is the number of characters in every Signal Code.
	CodeLength = 8
	// MaxSequence is the largest value representable by the four sequence characters.
	MaxSequence uint32 = 1<<24 - 1
	// EncodingAlphabet is the v0.1 64-character Signal Code alphabet.
	EncodingAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_"
)

var (
	ErrMalformedCode      = errors.New("signalbox: malformed Signal Code")
	ErrInvalidSeverity    = errors.New("signalbox: invalid severity")
	ErrInvalidIndex       = errors.New("signalbox: index is outside 0..63")
	ErrReservedField      = errors.New("signalbox: reserved character must be 0")
	ErrSequenceOutOfRange = errors.New("signalbox: sequence is outside 0..16777215")
	ErrUnassignedDomain   = errors.New("signalbox: domain index is unassigned")
	ErrUnassignedModule   = errors.New("signalbox: module index is unassigned in this domain")
	ErrUnknownSignal      = errors.New("signalbox: signal is not registered")
)

// Severity is the numeric severity identity encoded in the first Signal Code character.
type Severity uint8

const (
	SeverityNormal    Severity = 0
	SeverityWarning   Severity = 1
	SeverityMinor     Severity = 2
	SeverityMajor     Severity = 3
	SeverityCritical  Severity = 4
	SeverityEmergency Severity = 5
	SeverityHuman     Severity = 6
)

// Valid reports whether s is one of the seven v0.1 severity values.
func (s Severity) Valid() bool { return s <= SeverityHuman }

// SignalCode is an eight-character encoded signal identity.
// Use EncodeSignalCode or ParseSignalCode to create validated values.
type SignalCode string

// CodeComponents are the numeric fields decoded from a Signal Code.
type CodeComponents struct {
	Severity    Severity
	DomainIndex uint8
	ModuleIndex uint8
	Reserved    uint8
	Sequence    uint32
}

// EncodeDigit maps an index in 0..63 to its alphabet character.
func EncodeDigit(index uint8) (byte, error) {
	if index >= uint8(len(EncodingAlphabet)) {
		return 0, fmt.Errorf("%w: %d", ErrInvalidIndex, index)
	}
	return EncodingAlphabet[index], nil
}

// DecodeDigit maps one alphabet character to its numeric index.
func DecodeDigit(char byte) (uint8, error) {
	for index := 0; index < len(EncodingAlphabet); index++ {
		if EncodingAlphabet[index] == char {
			return uint8(index), nil
		}
	}
	return 0, fmt.Errorf("%w: invalid character %q", ErrMalformedCode, char)
}

// EncodeSignalCode encodes severity, domain, module and sequence using the v0.1 format.
func EncodeSignalCode(severity Severity, domainIndex, moduleIndex uint8, sequence uint32) (SignalCode, error) {
	if !severity.Valid() {
		return "", fmt.Errorf("%w: %d", ErrInvalidSeverity, severity)
	}
	if domainIndex >= 64 {
		return "", fmt.Errorf("domain: %w: %d", ErrInvalidIndex, domainIndex)
	}
	if moduleIndex >= 64 {
		return "", fmt.Errorf("module: %w: %d", ErrInvalidIndex, moduleIndex)
	}
	if sequence > MaxSequence {
		return "", fmt.Errorf("%w: %d", ErrSequenceOutOfRange, sequence)
	}

	var code [CodeLength]byte
	code[0] = EncodingAlphabet[severity]
	code[1] = EncodingAlphabet[domainIndex]
	code[2] = EncodingAlphabet[moduleIndex]
	code[3] = EncodingAlphabet[0]
	code[4] = EncodingAlphabet[(sequence>>18)&63]
	code[5] = EncodingAlphabet[(sequence>>12)&63]
	code[6] = EncodingAlphabet[(sequence>>6)&63]
	code[7] = EncodingAlphabet[sequence&63]
	return SignalCode(code[:]), nil
}

// ParseSignalCode checks the code's syntax and reserved field. It does not require a
// registered definition or an assigned domain/module.
func ParseSignalCode(raw string) (SignalCode, error) {
	code := SignalCode(raw)
	if _, err := DecodeSignalCode(code); err != nil {
		return "", err
	}
	return code, nil
}

// DecodeSignalCode parses the eight fields of code. Unknown but syntactically valid
// domain and module indices are preserved for registry-level validation.
func DecodeSignalCode(code SignalCode) (CodeComponents, error) {
	if len(code) != CodeLength {
		return CodeComponents{}, fmt.Errorf("%w: want %d characters, got %d", ErrMalformedCode, CodeLength, len(code))
	}
	var indexes [CodeLength]uint8
	for i := range code {
		index, err := DecodeDigit(code[i])
		if err != nil {
			return CodeComponents{}, fmt.Errorf("%w at position %d", err, i)
		}
		indexes[i] = index
	}
	if indexes[0] > uint8(SeverityHuman) {
		return CodeComponents{}, fmt.Errorf("%w: %d", ErrInvalidSeverity, indexes[0])
	}
	if indexes[3] != 0 {
		return CodeComponents{}, fmt.Errorf("%w: got %q", ErrReservedField, code[3])
	}
	sequence := uint32(indexes[4])<<18 | uint32(indexes[5])<<12 | uint32(indexes[6])<<6 | uint32(indexes[7])
	return CodeComponents{
		Severity:    Severity(indexes[0]),
		DomainIndex: indexes[1],
		ModuleIndex: indexes[2],
		Reserved:    indexes[3],
		Sequence:    sequence,
	}, nil
}

// Validate checks the code's syntax without checking whether its indices or signal are registered.
func (code SignalCode) Validate() error {
	_, err := DecodeSignalCode(code)
	return err
}

// String returns the encoded code unchanged.
func (code SignalCode) String() string { return string(code) }
