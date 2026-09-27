package signalbox

import (
	"errors"
	"testing"
)

func TestEncodingAlphabet(t *testing.T) {
	if len(EncodingAlphabet) != 64 {
		t.Fatalf("alphabet has %d characters, want 64", len(EncodingAlphabet))
	}
	seen := make(map[byte]bool, len(EncodingAlphabet))
	for index := 0; index < len(EncodingAlphabet); index++ {
		char := EncodingAlphabet[index]
		if seen[char] {
			t.Fatalf("duplicate alphabet character %q", char)
		}
		seen[char] = true
		gotIndex, err := DecodeDigit(char)
		if err != nil || int(gotIndex) != index {
			t.Errorf("DecodeDigit(%q) = %d, %v; want %d", char, gotIndex, err, index)
		}
		gotChar, err := EncodeDigit(uint8(index))
		if err != nil || gotChar != char {
			t.Errorf("EncodeDigit(%d) = %q, %v; want %q", index, gotChar, err, char)
		}
	}
	for _, test := range []struct {
		char  byte
		index uint8
	}{{'0', 0}, {'9', 9}, {'A', 10}, {'Z', 35}, {'a', 36}, {'z', 61}, {'-', 62}, {'_', 63}} {
		got, err := DecodeDigit(test.char)
		if err != nil || got != test.index {
			t.Errorf("DecodeDigit(%q) = %d, %v; want %d", test.char, got, err, test.index)
		}
	}
	if _, err := EncodeDigit(64); !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("EncodeDigit(64) error = %v, want ErrInvalidIndex", err)
	}
	if _, err := DecodeDigit('*'); !errors.Is(err, ErrMalformedCode) {
		t.Errorf("DecodeDigit('*') error = %v, want ErrMalformedCode", err)
	}
}

func TestSeverityValues(t *testing.T) {
	tests := []struct {
		severity Severity
		index    uint8
	}{
		{SeverityNormal, 0},
		{SeverityWarning, 1},
		{SeverityMinor, 2},
		{SeverityMajor, 3},
		{SeverityCritical, 4},
		{SeverityEmergency, 5},
		{SeverityHuman, 6},
	}
	for _, test := range tests {
		if uint8(test.severity) != test.index || !test.severity.Valid() {
			t.Errorf("severity %d has valid=%t; want %d and valid", test.severity, test.severity.Valid(), test.index)
		}
	}
	if Severity(7).Valid() {
		t.Error("severity 7 must be invalid")
	}
}

func TestEncodeDecodeSignalCode(t *testing.T) {
	code, err := EncodeSignalCode(SeverityMajor, 12, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := code.String(), "3C200001"; got != want {
		t.Fatalf("EncodeSignalCode = %q, want %q", got, want)
	}
	if len(code) != CodeLength {
		t.Fatalf("code length = %d, want %d", len(code), CodeLength)
	}
	parsed, err := ParseSignalCode(string(code))
	if err != nil {
		t.Fatal(err)
	}
	components, err := DecodeSignalCode(parsed)
	if err != nil {
		t.Fatal(err)
	}
	want := CodeComponents{Severity: SeverityMajor, DomainIndex: 12, ModuleIndex: 2, Reserved: 0, Sequence: 1}
	if components != want {
		t.Fatalf("decoded components = %#v, want %#v", components, want)
	}
}

func TestSequenceBoundaries(t *testing.T) {
	for _, sequence := range []uint32{0, 1, MaxSequence} {
		code, err := EncodeSignalCode(SeverityNormal, 0, 0, sequence)
		if err != nil {
			t.Fatalf("encode sequence %d: %v", sequence, err)
		}
		got, err := DecodeSignalCode(code)
		if err != nil {
			t.Fatalf("decode %s: %v", code, err)
		}
		if got.Sequence != sequence {
			t.Errorf("round trip sequence = %d, want %d", got.Sequence, sequence)
		}
	}
	zero, _ := EncodeSignalCode(SeverityNormal, 0, 0, 0)
	if got, want := zero, SignalCode("00000000"); got != want {
		t.Errorf("zero sequence code = %q, want %q", got, want)
	}
	max, _ := EncodeSignalCode(SeverityNormal, 0, 0, MaxSequence)
	if got, want := string(max[4:]), "____"; got != want {
		t.Errorf("maximum sequence digits = %q, want %q", got, want)
	}
	if _, err := EncodeSignalCode(SeverityNormal, 0, 0, MaxSequence+1); !errors.Is(err, ErrSequenceOutOfRange) {
		t.Errorf("overflow error = %v, want ErrSequenceOutOfRange", err)
	}
}

func TestRejectMalformedCodes(t *testing.T) {
	tests := []struct {
		code string
		want error
	}{
		{code: "3C20001", want: ErrMalformedCode},
		{code: "3C2000010", want: ErrMalformedCode},
		{code: "3C2*0001", want: ErrMalformedCode},
		{code: "3C210001", want: ErrReservedField},
		{code: "7C200001", want: ErrInvalidSeverity},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			if _, err := ParseSignalCode(test.code); !errors.Is(err, test.want) {
				t.Fatalf("ParseSignalCode(%q) error = %v, want %v", test.code, err, test.want)
			}
		})
	}
	if _, err := EncodeSignalCode(Severity(7), 12, 2, 1); !errors.Is(err, ErrInvalidSeverity) {
		t.Errorf("invalid severity error = %v", err)
	}
	if _, err := EncodeSignalCode(SeverityMajor, 64, 2, 1); !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("invalid domain index error = %v", err)
	}
	if _, err := EncodeSignalCode(SeverityMajor, 12, 64, 1); !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("invalid module index error = %v", err)
	}
}
