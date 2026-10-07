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
	if got := len(registry.Signals()); got != 0 {
		t.Errorf("canonical registry has %d application signals, want none", got)
	}
	for _, id := range switchyardSignalIDs {
		if _, err := registry.LookupID(id); !errors.Is(err, ErrUnknownSignal) {
			t.Errorf("LookupID(%q) = %v, want ErrUnknownSignal", id, err)
		}
	}
	if _, err := registry.LookupCode("3C200001"); !errors.Is(err, ErrUnknownSignal) {
		t.Errorf("3C200001 lookup = %v, want unregistered example code", err)
	}
}

var switchyardSignalIDs = []string{
	"invalid_chora_capacity",
	"invalid_reservation_count",
	"insufficient_free_hedra",
	"duplicate_allocation_id",
	"allocation_not_found",
	"invalid_hedra_state",
	"invalid_chora_state",
	"dispatcher_allocation_offering_mismatch",
	"dispatcher_provider_executor_missing",
	"dispatcher_provider_model_missing",
}

func TestLoadCanonicalNamespacesRetainsSwitchyardAssignments(t *testing.T) {
	namespaces, err := LoadCanonicalNamespaces()
	if err != nil {
		t.Fatal(err)
	}
	domainFound := false
	for _, domain := range namespaces.Domains {
		if domain.ID == "switchyard" && domain.Index == 12 {
			domainFound = true
		}
	}
	if !domainFound {
		t.Fatal("canonical namespaces do not include Switchyard domain index 12")
	}
	wantModules := map[string]uint8{"common": 0, "registry": 1, "chora": 2}
	for _, set := range namespaces.Modules {
		if set.DomainID != "switchyard" {
			continue
		}
		for _, module := range set.Modules {
			if want, ok := wantModules[module.ID]; ok {
				if module.Index != want {
					t.Errorf("Switchyard module %q index = %d, want %d", module.ID, module.Index, want)
				}
				delete(wantModules, module.ID)
			}
		}
	}
	if len(wantModules) != 0 {
		t.Errorf("missing Switchyard module assignments: %v", wantModules)
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
	byCode, err := registry.LookupCode("3A000001")
	if err != nil {
		t.Fatal(err)
	}
	if byCode.ID != "sample.local_condition" || byCode.Domain.Name != "QUINCUNX common/shared" || byCode.Module.Name != "Common" {
		t.Fatalf("lookup by code returned %#v", byCode)
	}
	byID, err := registry.LookupID(byCode.ID)
	if err != nil || byID.Code != byCode.Code {
		t.Fatalf("lookup by ID = %#v, %v", byID, err)
	}
	event, err := registry.NewEvent(byCode.ID, "Local condition", "sample-app", nil)
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
		{code: "3B000001", want: ErrUnassignedDomain},
		{code: "3A100001", want: ErrUnassignedModule},
		{code: "3A000002", want: ErrUnknownSignal},
		{code: "3C2*0001", want: ErrMalformedCode},
	} {
		if _, err := registry.LookupCode(SignalCode(test.code)); !errors.Is(err, test.want) {
			t.Errorf("LookupCode(%q) = %v, want %v", test.code, err, test.want)
		}
	}
	if err := registry.ValidateCode("3A000002"); err != nil {
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
				data.Domains = append(data.Domains, Domain{Index: 10, ID: "other", Name: "Other"})
			},
			want: ErrDuplicateIndex,
		},
		{
			name: "duplicate domain ID",
			mutate: func(data *RegistryData) {
				data.Domains = append(data.Domains, Domain{Index: 11, ID: "quincunx", Name: "Other"})
			},
			want: ErrDuplicateID,
		},
		{
			name: "duplicate module index",
			mutate: func(data *RegistryData) {
				data.Modules[0].Modules = append(data.Modules[0].Modules, Module{Index: 0, ID: "other", Name: "Other"})
			},
			want: ErrDuplicateIndex,
		},
		{
			name: "duplicate module ID",
			mutate: func(data *RegistryData) {
				data.Modules[0].Modules = append(data.Modules[0].Modules, Module{Index: 1, ID: "common", Name: "Other"})
			},
			want: ErrDuplicateID,
		},
		{
			name: "duplicate signal sequence and code",
			mutate: func(data *RegistryData) {
				duplicate := data.Signals[0]
				duplicate.ID = "sample.same_code"
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
		Domains:    []Domain{{Index: 10, ID: "quincunx", Name: "QUINCUNX common/shared"}},
		Modules: []ModuleSet{{DomainID: "quincunx", Modules: []Module{
			{Index: 0, ID: "common", Name: "Common"},
		}}},
		Signals: []SignalRecord{{
			ID: "sample.local_condition", Severity: SeverityMajor,
			DomainID: "quincunx", ModuleID: "common", Sequence: 1,
			Summary:     "Local condition",
			Diagnostics: []DiagnosticReference{{CapabilityID: "sample.inspect", Purpose: "Inspect the sample condition."}},
		}},
	}
}
