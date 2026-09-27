package signalbox

import (
	"errors"
	"os"
	"testing"
)

func TestLoadCanonicalRegistry(t *testing.T) {
	registry, err := LoadCanonicalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	severities := []string{"Normal", "Warning", "Minor", "Major", "Critical", "Emergency", "Human"}
	for index, want := range severities {
		definition, ok := registry.Severity(Severity(index))
		if !ok || definition.Name != want {
			t.Errorf("severity %d = %#v, present %t; want %q", index, definition, ok, want)
		}
	}
	if _, ok := registry.Severity(7); ok {
		t.Error("severity index 7 should not be assigned")
	}
	wantDomains := map[uint8]string{10: "quincunx", 11: "furnace", 12: "switchyard", 13: "puppeteer", 14: "airlock", 15: "foundry"}
	for index, want := range wantDomains {
		domain, ok := registry.DomainByIndex(index)
		if !ok || domain.ID != want {
			t.Errorf("domain %d = %#v, present %t; want ID %q", index, domain, ok, want)
		}
	}
	if _, ok := registry.DomainByIndex(16); ok {
		t.Error("reserved QUINCUNX expansion index 16 has an assignment")
	}
	for index, want := range map[uint8]string{0: "common", 1: "registry", 2: "chora"} {
		module, ok := registry.ModuleByIndex(12, index)
		if !ok || module.ID != want {
			t.Errorf("Switchyard module %d = %#v, present %t; want %q", index, module, ok, want)
		}
	}
	if _, ok := registry.ModuleByIndex(11, 2); ok {
		t.Error("Furnace module 2 should not be assigned")
	}
	wantSignals := map[string]string{
		"invalid_chora_capacity":    "4C200001",
		"invalid_reservation_count": "1C200002",
		"insufficient_free_hedra":   "1C200003",
		"duplicate_allocation_id":   "2C200004",
		"allocation_not_found":      "2C200005",
		"invalid_hedra_state":       "4C200006",
		"invalid_chora_state":       "4C200007",
	}
	seenCodes := make(map[SignalCode]string, len(wantSignals))
	for id, wantCode := range wantSignals {
		definition, err := registry.LookupID(id)
		if err != nil {
			t.Fatalf("LookupID(%q): %v", id, err)
		}
		if got := definition.Code.String(); got != wantCode {
			t.Errorf("%s code = %s, want %s", id, got, wantCode)
		}
		encoded, err := EncodeSignalCode(definition.Severity, definition.Domain.Index, definition.Module.Index, definition.Sequence)
		if err != nil || encoded != SignalCode(wantCode) {
			t.Errorf("encoding %s = %s, %v; want %s", id, encoded, err, wantCode)
		}
		if previous, exists := seenCodes[definition.Code]; exists {
			t.Errorf("signals %s and %s collide at %s", previous, id, definition.Code)
		}
		seenCodes[definition.Code] = id
		byCode, err := registry.LookupCode(definition.Code)
		if err != nil || byCode.ID != id {
			t.Errorf("LookupCode(%s) = %q, %v; want %q", definition.Code, byCode.ID, err, id)
		}
		if definition.Domain.ID != "switchyard" || definition.Domain.Index != 12 || definition.Module.ID != "chora" || definition.Module.Index != 2 {
			t.Errorf("%s domain/module = %#v/%#v, want Switchyard/Chora", id, definition.Domain, definition.Module)
		}
		if len(definition.Diagnostics) != 0 {
			t.Errorf("%s has diagnostic references %v, want none", id, definition.Diagnostics)
		}
	}
	if got := len(registry.Signals()); got != len(wantSignals) {
		t.Errorf("canonical registry has %d signals, want %d", got, len(wantSignals))
	}
	if _, err := registry.LookupCode("3C200001"); !errors.Is(err, ErrUnknownSignal) {
		t.Errorf("3C200001 lookup = %v, want unregistered example code", err)
	}
}

func TestLoadCanonicalRegistryFromDiskMatchesEmbedded(t *testing.T) {
	disk, err := LoadRegistry("../registry")
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := LoadCanonicalRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(disk.Signals()), len(embedded.Signals()); got != want {
		t.Fatalf("disk registry has %d signals, embedded registry has %d", got, want)
	}
}

func TestLoadRegistryFS(t *testing.T) {
	registry, err := LoadRegistryFS(os.DirFS(".."), "registry")
	if err != nil {
		t.Fatal(err)
	}
	if domain, ok := registry.DomainByID("switchyard"); !ok || domain.Index != 12 {
		t.Fatalf("FS-loaded Switchyard domain = %#v, present %t", domain, ok)
	}
}

func TestRegistryJSONRejectsUnknownFields(t *testing.T) {
	var collection struct {
		Signals []SignalRecord `json:"signals"`
	}
	err := decodeJSON([]byte(`{"signals":[],"code":"3C200001"}`), &collection)
	if err == nil {
		t.Fatal("registry loader accepted a manually supplied Signal Code field")
	}
}

func TestRegistryLookupsAndUnknownAssignments(t *testing.T) {
	registry, err := NewRegistry(validRegistryData())
	if err != nil {
		t.Fatal(err)
	}
	byCode, err := registry.LookupCode("3C200001")
	if err != nil {
		t.Fatal(err)
	}
	if byCode.ID != "switchyard.chora.insufficient_capacity" || byCode.Domain.Name != "Switchyard" || byCode.Module.Name != "Chora" {
		t.Fatalf("lookup by code returned %#v", byCode)
	}
	byID, err := registry.LookupID(byCode.ID)
	if err != nil || byID.Code != byCode.Code {
		t.Fatalf("lookup by ID = %#v, %v", byID, err)
	}
	event, err := registry.NewEvent(byCode.ID, "Insufficient capacity", "switchyard-main", map[string]any{"requested": 6})
	if err != nil || event.Code != byCode.Code || event.Timestamp.IsZero() {
		t.Fatalf("NewEvent = %#v, %v", event, err)
	}
	if _, err := registry.LookupID("missing.signal"); !errors.Is(err, ErrUnknownSignal) {
		t.Errorf("unknown ID error = %v, want ErrUnknownSignal", err)
	}
	if _, err := registry.NewEvent("missing.signal", "message", "source", nil); !errors.Is(err, ErrUnknownSignal) {
		t.Errorf("NewEvent for unknown ID error = %v, want ErrUnknownSignal", err)
	}
	for _, test := range []struct {
		code string
		want error
	}{
		{code: "3D200001", want: ErrUnassignedDomain},
		{code: "3C300001", want: ErrUnassignedModule},
		{code: "3C200002", want: ErrUnknownSignal},
		{code: "3C2*0001", want: ErrMalformedCode},
	} {
		if _, err := registry.LookupCode(SignalCode(test.code)); !errors.Is(err, test.want) {
			t.Errorf("LookupCode(%q) = %v, want %v", test.code, err, test.want)
		}
	}
	if err := registry.ValidateCode("3C200002"); err != nil {
		t.Errorf("assigned domain/module with unknown sequence should pass ValidateCode: %v", err)
	}
}

func TestRegistryValidationRejectsCollisionsAndInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RegistryData)
		want   error
	}{
		{
			name:   "duplicate severity index",
			mutate: func(data *RegistryData) { data.Severities = append(data.Severities, data.Severities[0]) },
			want:   ErrDuplicateIndex,
		},
		{
			name: "duplicate severity ID",
			mutate: func(data *RegistryData) {
				duplicate := data.Severities[1]
				duplicate.Index = 7
				data.Severities = append(data.Severities, duplicate)
			},
			want: ErrDuplicateID,
		},
		{
			name: "duplicate domain index",
			mutate: func(data *RegistryData) {
				data.Domains = append(data.Domains, Domain{Index: 12, ID: "other", Name: "Other"})
			},
			want: ErrDuplicateIndex,
		},
		{
			name: "duplicate domain ID",
			mutate: func(data *RegistryData) {
				data.Domains = append(data.Domains, Domain{Index: 13, ID: "switchyard", Name: "Other"})
			},
			want: ErrDuplicateID,
		},
		{
			name: "duplicate module index",
			mutate: func(data *RegistryData) {
				data.Modules[0].Modules = append(data.Modules[0].Modules, Module{Index: 2, ID: "other", Name: "Other"})
			},
			want: ErrDuplicateIndex,
		},
		{
			name: "duplicate module ID",
			mutate: func(data *RegistryData) {
				data.Modules[0].Modules = append(data.Modules[0].Modules, Module{Index: 3, ID: "chora", Name: "Other"})
			},
			want: ErrDuplicateID,
		},
		{
			name: "duplicate signal sequence and code",
			mutate: func(data *RegistryData) {
				duplicate := data.Signals[0]
				duplicate.ID = "switchyard.chora.same_code"
				data.Signals = append(data.Signals, duplicate)
			},
			want: ErrDuplicateSequence,
		},
		{
			name: "duplicate signal ID",
			mutate: func(data *RegistryData) {
				duplicate := data.Signals[0]
				duplicate.Sequence = 2
				data.Signals = append(data.Signals, duplicate)
			},
			want: ErrDuplicateID,
		},
		{
			name:   "out of range domain index",
			mutate: func(data *RegistryData) { data.Domains[0].Index = 64 },
			want:   ErrInvalidIndex,
		},
		{
			name:   "out of range module index",
			mutate: func(data *RegistryData) { data.Modules[0].Modules[0].Index = 64 },
			want:   ErrInvalidIndex,
		},
		{
			name:   "out of range sequence",
			mutate: func(data *RegistryData) { data.Signals[0].Sequence = MaxSequence + 1 },
			want:   ErrSequenceOutOfRange,
		},
		{
			name:   "nonzero reserved field",
			mutate: func(data *RegistryData) { data.Signals[0].Reserved = 1 },
			want:   ErrReservedField,
		},
		{
			name:   "malformed diagnostic reference",
			mutate: func(data *RegistryData) { data.Signals[0].Diagnostics[0].CapabilityID = "Not A Capability" },
			want:   ErrInvalidDefinition,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := validRegistryData()
			test.mutate(&data)
			_, err := NewRegistry(data)
			if !errors.Is(err, test.want) {
				t.Fatalf("NewRegistry error = %v, want %v", err, test.want)
			}
			if test.name == "duplicate signal sequence and code" && !errors.Is(err, ErrDuplicateCode) {
				t.Fatalf("duplicate final code error = %v, want ErrDuplicateCode", err)
			}
		})
	}
}

func validRegistryData() RegistryData {
	severityNames := []string{"Normal", "Warning", "Minor", "Major", "Critical", "Emergency", "Human"}
	severityIDs := []string{"normal", "warning", "minor", "major", "critical", "emergency", "human"}
	severities := make([]SeverityDefinition, len(severityNames))
	for i := range severityNames {
		severities[i] = SeverityDefinition{Index: uint8(i), ID: severityIDs[i], Name: severityNames[i]}
	}
	return RegistryData{
		Severities: severities,
		Domains:    []Domain{{Index: 12, ID: "switchyard", Name: "Switchyard"}},
		Modules: []ModuleSet{{DomainID: "switchyard", Modules: []Module{
			{Index: 0, ID: "common", Name: "Common"},
			{Index: 1, ID: "registry", Name: "Registry"},
			{Index: 2, ID: "chora", Name: "Chora"},
		}}},
		Signals: []SignalRecord{{
			ID: "switchyard.chora.insufficient_capacity", Severity: SeverityMajor,
			DomainID: "switchyard", ModuleID: "chora", Sequence: 1,
			Summary:     "Insufficient Chora capacity",
			Diagnostics: []DiagnosticReference{{CapabilityID: "switchyard.chora.inspect", Purpose: "Inspect current Chora capacity and allocation state."}},
		}},
	}
}
